/*
 * Copyright © 2018 Knative Authors (knative-dev@googlegroups.com)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package dev.knative.eventing.kafka.broker.receiverloom;

import dev.knative.eventing.kafka.broker.core.ReactiveKafkaProducer;
import dev.knative.eventing.kafka.broker.core.tracing.kafka.ProducerTracer;
import io.vertx.core.Future;
import io.vertx.core.Promise;
import io.vertx.core.Vertx;
import io.vertx.core.impl.ContextInternal;
import io.vertx.core.impl.VertxInternal;
import java.util.Queue;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.concurrent.atomic.AtomicBoolean;
import org.apache.kafka.clients.producer.Producer;
import org.apache.kafka.clients.producer.ProducerRecord;
import org.apache.kafka.clients.producer.RecordMetadata;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class LoomKafkaProducer<K, V> implements ReactiveKafkaProducer<K, V> {

    private static final Logger logger = LoggerFactory.getLogger(LoomKafkaProducer.class);

    private final Producer<K, V> producer;

    private final Queue<RecordPromise> queue;
    private final AtomicBoolean isRunning;
    private final ProducerTracer tracer;
    private final VertxInternal vertx;
    private final ContextInternal ctx;

    public LoomKafkaProducer(Vertx v, Producer<K, V> producer) {
        this.producer = producer;
        this.queue = new ConcurrentLinkedQueue<>();
        this.isRunning = new AtomicBoolean(false);
        this.vertx = (VertxInternal) v;

        if (v != null) {
            ContextInternal ctxInt = ((ContextInternal) v.getOrCreateContext()).unwrap();
            this.tracer = ProducerTracer.create(ctxInt.tracer());
            this.ctx = vertx.getOrCreateContext();
        } else {
            this.tracer = null;
            this.ctx = null;
        }
    }

    @Override
    public Future<RecordMetadata> send(ProducerRecord<K, V> record) {
        Promise<RecordMetadata> promise = Promise.promise();
        queue.add(new RecordPromise(record, promise));
        if (isRunning.compareAndSet(false, true)) {
            Thread.ofVirtual().start(this::sendFromQueue).setPriority(Thread.MAX_PRIORITY);
        }
        return promise.future();
    }

    private void sendFromQueue() {
        while (!queue.isEmpty()) {
            RecordPromise recordPromise = queue.poll();
            ProducerTracer.StartedSpan startedSpan =
                    this.tracer == null ? null : this.tracer.prepareSendMessage(ctx, recordPromise.getRecord());
            logger.info("Mylog, span created: {}", startedSpan);
            try {
                var metadata = producer.send(recordPromise.getRecord());
                logger.info("Mylog, send complete: {}", metadata);
                recordPromise.getPromise().complete(metadata.get());
                if (startedSpan != null) {
                    startedSpan.finish(ctx);
                }
            } catch (Exception e) {
                recordPromise.getPromise().fail(e);
                if (startedSpan != null) {
                    startedSpan.fail(ctx, e);
                }
            }
        }
        isRunning.set(false);
    }

    @Override
    public Future<Void> close() {
        Promise<Void> promise = Promise.promise();
        Thread.ofVirtual().start(() -> {
            try {
                producer.close();
                promise.complete();
            } catch (Exception e) {
                promise.fail(e);
            }
        });
        return promise.future();
    }

    @Override
    public Future<Void> flush() {
        Promise<Void> promise = Promise.promise();
        Thread.ofVirtual().start(() -> {
            try {
                producer.flush();
                promise.complete();
            } catch (Exception e) {
                promise.fail(e);
            }
        });
        return promise.future();
    }

    @Override
    public Producer<K, V> unwrap() {
        return producer;
    }

    private class RecordPromise {
        private final ProducerRecord<K, V> record;
        private final Promise<RecordMetadata> promise;

        private RecordPromise(ProducerRecord<K, V> record, Promise<RecordMetadata> promise) {
            this.record = record;
            this.promise = promise;
        }

        public ProducerRecord<K, V> getRecord() {
            return record;
        }

        public Promise<RecordMetadata> getPromise() {
            return promise;
        }
    }
}