/*
 * Copyright (c) 2020 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/golang/glog"
	"l7e.io/vanity"
	"l7e.io/vanity/cmd/vanity/cli/backends"
	"l7e.io/vanity/cmd/vanity/server/interceptors"
	"l7e.io/vanity/pkg/toml"
	"l7e.io/yama"
)

func main() {
	flag.Parse()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	glog.Infof("port configured to listen to %s", addr)

	file := os.Getenv("TOML_FILE")
	if file == "" {
		file = "/var/vanity/config.toml"
	}
	glog.Infof("TOML file location %s", file)

	table := os.Getenv("TOML_TABLE")
	if table == "" {
		table = "vanity"
	}
	glog.Infof("TOML table name %s", table)

	be, err := toml.NewTOMLBackend(toml.InTable(table), toml.FromFile(file))
	if err != nil {
		glog.Fatal(err)
	}

	glog.Info("Vanity entries:")
	err = be.List(context.Background(), vanity.ConsumerFunc(func(context context.Context, importPath, vcs, vcsPath string) {
		glog.Infof("  %s %s %s", importPath, vcs, vcsPath)
	}))
	if err != nil {
		glog.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", interceptors.WrapHandler(vanity.NewVanityHandler(be)))

	s := &http.Server{Addr: addr, Handler: mux}

	watcher := yama.NewWatcher(
		yama.WatchingSignals(syscall.SIGINT, syscall.SIGTERM),
		yama.WithTimeout(2*time.Second), // nolint
		yama.WithClosers(backends.Backend, s))

	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		glog.Error(err)
		_ = watcher.Close()
	}

	if err := watcher.Wait(); err != nil {
		glog.Warningf("Shutdown error: %s", err)
	}
}
