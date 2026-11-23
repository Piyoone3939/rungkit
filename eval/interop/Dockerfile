FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential autoconf automake libtool flex bison git ca-certificates \
      python3 python3-lxml python3-jinja2 python3-click python3-msgpack python3-zeroconf \
      python3-pycountry python3-autobahn python3-twisted python3-openssl \
 && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 https://github.com/beremiz/matiec /opt/matiec \
 && cd /opt/matiec && autoreconf -i && ./configure && make -j4
RUN git clone --depth 1 https://github.com/beremiz/beremiz /opt/beremiz
WORKDIR /work
