FROM scratch

COPY rastermimimi /
COPY etc/passwd /etc/passwd

USER nobody

ENTRYPOINT ["/rastermimimi"]
