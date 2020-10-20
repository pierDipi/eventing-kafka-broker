package dev.knative.eventing.kafka.broker.dispatcher.http;

import io.cloudevents.core.builder.CloudEventBuilder;
import io.cloudevents.core.message.MessageReader;
import io.cloudevents.http.vertx.VertxMessageFactory;
import io.vertx.core.Vertx;
import io.vertx.ext.web.client.WebClient;
import io.vertx.junit5.VertxExtension;
import io.vertx.junit5.VertxTestContext;
import java.net.URI;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import org.assertj.core.api.Assertions;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(VertxExtension.class)
public class ReplyToURLResponseHandlerTest {

  private final static int PORT = 64789;


  // client request (event) -> server response (event) -> client response handled by CUT.
  @Test
  public void shouldSendReply(final Vertx vertx, final VertxTestContext context) throws InterruptedException {

    final var cp = context.checkpoint(2);

    final var waitServer = new CountDownLatch(1);

    final var responseEvent = CloudEventBuilder.v1()
      .withId(UUID.randomUUID().toString())
      .withType("type1")
      .withSource(URI.create("/knative"))
      .build();

    final var server = vertx.createHttpServer()
      .exceptionHandler(context::failNow)
      .requestHandler(request -> VertxMessageFactory.createReader(request)
        .map(MessageReader::toEvent)
        .onFailure(context::failNow)
        .onSuccess(event -> {

          context.verify(() -> {
            Assertions.assertThat(event).isEqualTo(responseEvent);
            cp.flag();
          });

          VertxMessageFactory
            .createWriter(request.response())
            .writeBinary(responseEvent);
        })
      );

    server
      .listen(PORT, "127.0.0.1")
      .onFailure(context::failNow)
      .onSuccess(ignore -> waitServer.countDown());

    waitServer.await();

    final var replyURL = "http://127.0.0.1:" + PORT;

    final var client = WebClient.create(vertx);

    VertxMessageFactory
      .createWriter(client.postAbs(replyURL))
      .writeBinary(responseEvent)
      .compose(response -> new ReplyToURLResponseHandler(client, replyURL)
        .handle(response)
        .onFailure(context::failNow)
        .onSuccess(ignored -> {
          client.close();
          server.close();

          cp.flag();
        })
      )
      .onFailure(context::failNow);
  }

  @Test
  public void nonAbsoluteURLsAreNotAccepted(final Vertx vertx) {

    final var client = WebClient.create(vertx);

    Assertions.assertThatThrownBy(
      () -> new ReplyToURLResponseHandler(client, "/knative")
    );

    client.close();
  }
}
