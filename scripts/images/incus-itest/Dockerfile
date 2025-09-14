# Incus-enabled image for running integration tests in a privileged container
# Base: Ubuntu 24.04 (Noble) with systemd, Go 1.22 (from Ubuntu), and Incus.

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Core tools and systemd
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        systemd systemd-sysv dbus udev \
        ca-certificates curl gnupg git sudo \
        tzdata \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Add Zabbly Incus stable repo and install Incus + client
RUN set -eux; \
    mkdir -p /usr/share/keyrings; \
    curl -fsSL https://pkgs.zabbly.com/key.asc | gpg --dearmor -o /usr/share/keyrings/zabbly.gpg; \
    . /etc/os-release; \
    echo "deb [signed-by=/usr/share/keyrings/zabbly.gpg] https://pkgs.zabbly.com/incus/stable/ $VERSION_CODENAME main" \
      > /etc/apt/sources.list.d/zabbly-incus-stable.list; \
    apt-get update; \
    apt-get install -y --no-install-recommends incus incus-client; \
    apt-get clean; rm -rf /var/lib/apt/lists/*

# Install Go (Ubuntu noble ships Go 1.22)
RUN apt-get update \
    && apt-get install -y --no-install-recommends golang-go \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Drop a helper entrypoint that can be used for ad-hoc exec
COPY --chmod=755 run-tests.sh /usr/local/bin/run-tests.sh

# Default to systemd init so services are available in the container
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]

