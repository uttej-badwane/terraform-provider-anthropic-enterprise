// Command mockserver runs the in-memory Admin API mock on a fixed port so the
// provider can be exercised with the real terraform / tofu CLI:
//
//	go run ./internal/mock/cmd/mockserver -addr 127.0.0.1:8787
//	export ANTHROPIC_BASE_URL=http://127.0.0.1:8787 ANTHROPIC_ADMIN_API_KEY=<printed>
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/mock"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address")
	flag.Parse()

	srv := mock.NewServer() // starts on a random port; we re-serve its handler on addr
	defer srv.Close()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("mock Admin API listening on http://%s\n", ln.Addr())
	fmt.Printf("export ANTHROPIC_BASE_URL=http://%s\n", ln.Addr())
	fmt.Printf("export ANTHROPIC_ADMIN_API_KEY=%s\n", mock.AdminKey)
	fmt.Printf("export ANTHROPIC_AUTH_TOKEN=%s\n", mock.OAuthToken)
	fmt.Printf("export ANTHROPIC_ENTERPRISE_API_KEY=%s\n", mock.EnterpriseKey)

	go func() {
		_ = http.Serve(ln, srv.Config.Handler)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
}
