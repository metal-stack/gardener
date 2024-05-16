// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/pflag"
)

const (
	// IgnoreOperationAnnotationFlag is the name of the command line flag to specify whether the operation annotation
	// is ignored or not.
	IgnoreOperationAnnotationFlag = "ignore-operation-annotation"
	// ExtensionClassFlag TODO
	ExtensionClassFlag = "extension-class"
)

// ReconcilerOptions are command line options that can be set for controller.Options.
type ReconcilerOptions struct {
	// IgnoreOperationAnnotation defines whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool

	// ExtensionClass TODO
	ExtensionClass string

	config *ReconcilerConfig
}

// AddFlags implements Flagger.AddFlags.
func (c *ReconcilerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&c.IgnoreOperationAnnotation, IgnoreOperationAnnotationFlag, c.IgnoreOperationAnnotation, "Ignore the operation annotation or not.")
	fs.StringVar(&c.ExtensionClass, ExtensionClassFlag, "", "The class that this extension will watch.")
}

// Complete implements Completer.Complete.
func (c *ReconcilerOptions) Complete() error {
	c.config = &ReconcilerConfig{c.IgnoreOperationAnnotation, c.ExtensionClass}
	return nil
}

// Completed returns the completed ReconcilerConfig. Only call this if `Complete` was successful.
func (c *ReconcilerOptions) Completed() *ReconcilerConfig {
	return c.config
}

// ReconcilerConfig is a completed controller configuration.
type ReconcilerConfig struct {
	// IgnoreOperationAnnotation defines whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// ExtensionClass TODO
	ExtensionClass string
}

// Apply sets the values of this ReconcilerConfig in the given controller.Options.
func (c *ReconcilerConfig) Apply(ignore *bool) {
	*ignore = c.IgnoreOperationAnnotation
}

// Apply sets the values of this ReconcilerConfig in the given controller.Options.
func (c *ReconcilerConfig) ApplyClass(class *string) {
	*class = c.ExtensionClass
}
