package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"
)

var tryCloudflareURL = regexp.MustCompile(`https://[A-Za-z0-9-]+\.trycloudflare\.com`)
var cloudflareReadyLine = regexp.MustCompile(`Registered tunnel connection|Route propagating|Connection .* registered`)

type Options struct {
	Hostname string
	Name     string
	TokenEnv string
}

type Session interface {
	URL() string
	Close(context.Context) error
}

type CloudflareQuick struct {
	Command string
	Timeout time.Duration
}

type CloudflareSession struct {
	url    string
	cancel context.CancelFunc
	done   <-chan error
	once   sync.Once
	err    error
}

type tailBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if len(b.data) > b.limit {
		b.data = b.data[len(b.data)-b.limit:]
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

func (c CloudflareQuick) Start(ctx context.Context, localURL string, opts Options) (Session, error) {
	command := c.Command
	if command == "" {
		command = "cloudflared"
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("%s is not installed; install cloudflared and retry", command)
	}

	args, env, err := cloudflaredArgs(localURL, opts)
	if err != nil {
		return nil, err
	}

	startCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(startCtx, path, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var outputReader *io.PipeReader
	var outputWriter *io.PipeWriter
	var tokenOutput *tailBuffer
	if opts.TokenEnv == "" {
		outputReader, outputWriter = io.Pipe()
		cmd.Stdout = outputWriter
		cmd.Stderr = outputWriter
	} else {
		tokenOutput = newTailBuffer(4096)
		cmd.Stdout = tokenOutput
		cmd.Stderr = tokenOutput
	}

	if err := cmd.Start(); err != nil {
		cancel()
		if outputWriter != nil {
			_ = outputWriter.Close()
		}
		return nil, err
	}

	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		if outputWriter != nil {
			_ = outputWriter.Close()
		}
		done <- err
	}()

	timeout := c.Timeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}
	if opts.TokenEnv != "" {
		if err := waitForTokenTunnel(done, timeout, tokenOutput); err != nil {
			cancel()
			return nil, err
		}
		return &CloudflareSession{url: "https://" + opts.Hostname, cancel: cancel, done: done}, nil
	}
	url, err := waitForURL(outputReader, done, timeout, opts.Hostname)
	if err != nil {
		cancel()
		<-done
		return nil, err
	}
	return &CloudflareSession{url: url, cancel: cancel, done: done}, nil
}

func waitForTokenTunnel(done <-chan error, timeout time.Duration, output fmt.Stringer) error {
	ready := 3 * time.Second
	if timeout > 0 && timeout < ready {
		ready = timeout
	}
	timer := time.NewTimer(ready)
	defer timer.Stop()
	select {
	case err := <-done:
		if err == nil {
			err = errors.New("cloudflared exited before the token tunnel stayed ready")
		}
		if output != nil && output.String() != "" {
			return fmt.Errorf("%w: %s", err, output.String())
		}
		return err
	case <-timer.C:
		return nil
	}
}

func cloudflaredArgs(localURL string, opts Options) ([]string, []string, error) {
	if opts.TokenEnv != "" {
		token := os.Getenv(opts.TokenEnv)
		if token == "" {
			return nil, nil, fmt.Errorf("%s is not set", opts.TokenEnv)
		}
		return []string{"tunnel", "--no-autoupdate", "run", "--url", localURL}, []string{"TUNNEL_TOKEN=" + token}, nil
	}

	args := []string{"tunnel", "--no-autoupdate", "--url", localURL}
	if opts.Hostname != "" {
		args = append(args, "--hostname", opts.Hostname)
	}
	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}
	return args, nil, nil
}

func (s *CloudflareSession) URL() string {
	return s.url
}

func (s *CloudflareSession) Close(ctx context.Context) error {
	s.once.Do(func() {
		s.cancel()
		select {
		case err := <-s.done:
			if err != nil && !errors.Is(err, context.Canceled) {
				s.err = err
			}
		case <-ctx.Done():
			s.err = ctx.Err()
		}
	})
	return s.err
}

func waitForURL(output io.Reader, done <-chan error, timeout time.Duration, hostname string) (string, error) {
	type scanResult struct {
		url string
		err error
	}
	results := make(chan scanResult, 1)
	go func() {
		url, err := scanForURL(output, hostname)
		results <- scanResult{url: url, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-results:
		if result.err != nil {
			return "", result.err
		}
		return result.url, nil
	case err := <-done:
		if err == nil {
			err = errors.New("cloudflared exited before publishing a tunnel URL")
		}
		return "", err
	case <-timer.C:
		return "", errors.New("timed out waiting for Cloudflare tunnel URL")
	}
}

func scanForURL(output io.Reader, hostname string) (string, error) {
	var recent bytes.Buffer
	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		line := scanner.Text()
		if match := tryCloudflareURL.FindString(line); match != "" {
			return match, nil
		}
		if hostname != "" && cloudflareReadyLine.MatchString(line) {
			return "https://" + hostname, nil
		}
		if recent.Len() < 4096 {
			recent.WriteString(line)
			recent.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if recent.Len() > 0 {
		if hostname != "" {
			return "", fmt.Errorf("cloudflared did not confirm the custom hostname tunnel: %s", recent.String())
		}
		return "", fmt.Errorf("cloudflared did not publish a trycloudflare URL: %s", recent.String())
	}
	if hostname != "" {
		return "", errors.New("cloudflared did not confirm the custom hostname tunnel")
	}
	return "", errors.New("cloudflared did not publish a trycloudflare URL")
}
