# fdstream
# DRAFT
Simple sync and async wrapper for streaming data via io.Reader and Writer (Most likely TCP conn)
It can be any process Std* or TCP connection or just file or Unix socket.

The adventure of using package is simple way for communicate without handshake. In case HTTPS or TLS it can take time. 

# Example

## Async

Sometimes need to just send data without any response from second side. It can be log or statistic collecting. Limitation since we read data with dynamic size we can't read it by multiple readers. We should read it in singleton. But we still free write data concurrently (in case io.Writer support concurrent writing).

## Sync
It is a way to send data and expect response.

Sync detect message by id *Message.id*

