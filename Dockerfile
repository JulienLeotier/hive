# Multi-arch Docker image for the Hive binary produced by GoReleaser.
FROM gcr.io/distroless/static:nonroot

COPY hive /usr/local/bin/hive

EXPOSE 7777
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/hive"]
CMD ["serve"]
