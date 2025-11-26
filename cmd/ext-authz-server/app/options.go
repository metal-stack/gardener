// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/spf13/pflag"
)

type options struct {
	logLevel   string
	logFormat  string
	port       int
	secretsDir string
	reflection bool
}

func (o *options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.logLevel, "log-level", "info", "Log level")
	fs.StringVar(&o.logFormat, "log-format", "text", "Log format")
	fs.IntVar(&o.port, "port", 10000, "Port of grpc server")
	fs.StringVar(&o.secretsDir, "secrets-dir", "/secrets", "dir holding secrets")
	fs.BoolVar(&o.reflection, "grpc-reflection", false, "enable grpc reflection")
}

func (o *options) Complete() error {
	return nil
}

func (o *options) Validate() error {
	return nil
}

func (o *options) LogConfig() (string, string) {
	return o.logLevel, o.logFormat
}
