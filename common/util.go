package common

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func WaitFor[T comparable](ch <-chan T, val T) bool {
	for curr := range ch {
		if curr == val {
			return true
		}
	}

	return false
}

func NewSignalCtx(
	ctx context.Context,
) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)
	signal.Notify(stop, syscall.SIGINT)

	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-stop:
			cancel()
		}
	}()

	return ctx
}

// chatgpt generated
func FlushWithContext(context string, into io.Writer, from io.ReadCloser) {
	defer from.Close()

	var buf bytes.Buffer
	tmp := make([]byte, 1)

	for {
		n, err := from.Read(tmp)
		if n > 0 {
			c := tmp[0]
			buf.WriteByte(c)

			if c == '\n' || c == '\r' {
				line := buf.Bytes()

				if c == '\r' {
					fmt.Fprintf(into, "\r%s: %s", context, string(line[:len(line)-1]))
				} else {
					fmt.Fprintf(into, "\r%s: %s", context, string(line))
				}

				buf.Reset()
			}
		}

		if err == io.EOF {
			if buf.Len() > 0 {
				fmt.Fprintf(into, "%s: %s", context, buf.String())
			}
			break
		}

		if err != nil {
			break
		}
	}
}
