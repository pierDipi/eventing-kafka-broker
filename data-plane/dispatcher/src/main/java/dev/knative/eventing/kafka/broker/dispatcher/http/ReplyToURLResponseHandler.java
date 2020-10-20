package dev.knative.eventing.kafka.broker.dispatcher.http;

import dev.knative.eventing.kafka.broker.dispatcher.SinkResponseHandler;
import io.cloudevents.http.vertx.VertxMessageFactory;
import io.vertx.core.Future;
import io.vertx.core.buffer.Buffer;
import io.vertx.ext.web.client.HttpResponse;
import io.vertx.ext.web.client.WebClient;
import java.net.URI;
import java.util.Objects;

public final class ReplyToURLResponseHandler implements SinkResponseHandler<HttpResponse<Buffer>> {

  private final WebClient client;
  private final String replyUrl;

  public ReplyToURLResponseHandler(final WebClient client, final String replyURL) {
    Objects.requireNonNull(client, "provide client");
    Objects.requireNonNull(replyURL, "provide replyURL");

    if (!URI.create(replyURL).isAbsolute()) {
      throw new IllegalArgumentException("replyURL must be an absolute URL");
    }

    this.client = client;
    this.replyUrl = replyURL;
  }

  @Override
  public Future<Void> handle(final HttpResponse<Buffer> response) {

    return VertxMessageFactory
      .createWriter(client.postAbs(replyUrl))
      .writeBinary(
        VertxMessageFactory
          .createReader(response)
          .toEvent()
      )
      // TODO what should we do with the response?
      .mapEmpty();
  }
}
