FROM postgres:16

RUN apt-get update && apt-get install -y --no-install-recommends openssl \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /etc/ssl/postgresql \
    && openssl req -new -x509 -days 3650 -nodes \
         -subj "/CN=localhost" \
         -keyout /etc/ssl/postgresql/server.key \
         -out  /etc/ssl/postgresql/server.crt \
    && chmod 600 /etc/ssl/postgresql/server.key \
    && chown postgres:postgres /etc/ssl/postgresql/server.key /etc/ssl/postgresql/server.crt

COPY postgresql.conf /etc/postgresql/postgresql.conf
