package common

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

func WaitFor[T comparable](ch <-chan T, close func(), vals ...T) *T {
	return WaitForFn(ch, close, func(curr T) bool {
		for _, v := range vals {
			if v == curr {
				return true
			}
		}

		return false
	})
}

func WaitForFn[T any](ch <-chan T, close func(), fn func(T) bool) *T {
	defer func() {
		go func() {
			for range ch {
			}
		}()
		close()
	}()

	for curr := range ch {
		if fn(curr) {
			return &curr
		}
	}

	return nil
}

func ChWithInitial[T any](ch chan T, initial T) chan T {
	ret := make(chan T)

	go func() {
		ret <- initial

		for v := range ch {
			ret <- v
		}
	}()

	return ret
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
func FlushWithContext(
	context string, into io.Writer, from io.ReadCloser,
) {
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
					fmt.Fprintf(into, "\r%s %s", context, string(line[:len(line)-1]))
				} else {
					fmt.Fprintf(into, "\r%s %s", context, string(line))
				}

				buf.Reset()
			}
		}

		if err == io.EOF {
			if buf.Len() > 0 {
				fmt.Fprintf(into, "%s %s", context, buf.String())
			}
			break
		}

		if err != nil {
			break
		}
	}
}

// chatgpt generated
func StreamLines(readClosers ...io.ReadCloser) io.ReadCloser {
	pr, pw := io.Pipe()

	go func() {
		var wg sync.WaitGroup
		ch := make(chan string)

		for _, r := range readClosers {
			wg.Add(1)
			go func(r io.ReadCloser) {
				defer wg.Done()
				defer r.Close()
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					ch <- scanner.Text()
				}
			}(r)
		}

		go func() {
			wg.Wait()
			close(ch)
		}()

		for line := range ch {
			_, err := fmt.Fprintln(pw, line)
			if err != nil {
				pw.CloseWithError(err)
				return
			}
		}

		pw.Close()
	}()

	return pr
}

func GetVarDir(pid int) string {
	if uid := syscall.Getuid(); uid == 0 {
		return fmt.Sprintf("/var/run/ella/%d", pid)
	} else {
		return fmt.Sprintf(
			"/var/run/user/%d/ella/%d", uid, pid,
		)
	}
}

func WaitAny(
	ctx context.Context, fns ...func(ctx context.Context) error,
) {
	var wg sync.WaitGroup
	wg.Add(len(fns) + 1)

	cancels := make(chan struct{})
	subCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer wg.Done()

		<-cancels
		cancel()

		for i := 0; i < len(fns)-1; i++ {
			<-cancels
		}
		close(cancels)
	}()

	for _, fn := range fns {
		go func() {
			defer wg.Done()

			err := fn(subCtx)
			if err != nil {
				fmt.Println("error:", err)
			}

			cancels <- struct{}{}
		}()
	}

	wg.Wait()
}

func ShellEscape(arg string) string {
	if arg == "" {
		return "''"
	}

	safeChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_@%+=:,./-"
	for _, r := range arg {
		if !strings.ContainsRune(safeChars, r) {
			return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		}
	}
	return arg
}

func GetJsonSchemaAddress(version string) string {
	if version == "dev" {
		return "https://raw.githubusercontent.com/TheKhanj/ella/refs/heads/master/schema.json"
	}

	tmp := "https://raw.githubusercontent.com/TheKhanj/ella/refs/tags/%s/schema.json"
	return fmt.Sprintf(tmp, version)
}
