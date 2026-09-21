# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o openai-emulator cmd/openai-emulator/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

# Not /root, and world-executable: OpenShift's restricted SCC runs a container
# as an arbitrary UID that is not root and not in root's group, which cannot
# traverse /root (0700). A binary left there fails to start with
#   exec container process `/root/./openai-emulator`: Permission denied
# and the pod CrashLoopBackOffs with nothing else to go on.
COPY --from=builder /app/openai-emulator /usr/local/bin/openai-emulator
RUN chmod 0755 /usr/local/bin/openai-emulator

# The emulator keeps its state in memory and writes nothing, so it needs no
# writable directory - but it must not default to one it cannot read either.
WORKDIR /tmp

# An arbitrary non-root UID, so the image behaves the same way locally as it
# does under a restricted SCC. OpenShift overrides this with its own UID from
# the namespace range; the point is that neither UID needs to be root.
USER 1001

# Expose port
EXPOSE 8080

# Run the binary
CMD ["openai-emulator"]
