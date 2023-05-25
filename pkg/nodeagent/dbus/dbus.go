package dbus

import (
	"context"
	"fmt"

	"github.com/coreos/go-systemd/v22/dbus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

const done = "done"

func Enable(ctx context.Context, unitNames ...string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	_, _, err = dbc.EnableUnitFilesContext(ctx, unitNames, false, true)

	return err
}

func Disable(ctx context.Context, unitNames ...string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	_, err = dbc.DisableUnitFilesContext(ctx, unitNames, false)
	return err
}

func Stop(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	c := make(chan string)

	_, err = dbc.StopUnitContext(ctx, unitName, "replace", c)

	job := <-c
	if job != done {
		err = fmt.Errorf("stop failed %s", job)
	}

	recordEvent(recorder, node, err, unitName, "SystemDUnitStop", "stop")

	return err
}

func Start(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	c := make(chan string)

	_, err = dbc.StartUnitContext(ctx, unitName, "replace", c)
	if err != nil {
		return err
	}

	job := <-c
	if job != done {
		err = fmt.Errorf("start failed %s", job)
	}

	recordEvent(recorder, node, err, unitName, "SystemDUnitStart", "start")

	return err
}

func Restart(ctx context.Context, recorder record.EventRecorder, node *corev1.Node, unitName string) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	c := make(chan string)

	_, err = dbc.RestartUnitContext(ctx, unitName, "replace", c)
	if err != nil {
		return err
	}

	job := <-c
	if job != done {
		err = fmt.Errorf("restart failed %s", job)
	}

	recordEvent(recorder, node, err, unitName, "SystemDUnitRestart", "restart")

	return nil
}

func DaemonReload(ctx context.Context) error {
	dbc, err := dbus.NewWithContext(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to dbus: %w", err)
	}
	defer dbc.Close()

	err = dbc.ReloadContext(ctx)
	if err != nil {
		return fmt.Errorf("systemd daemon-reload failed %w", err)
	}

	return nil
}

func recordEvent(recorder record.EventRecorder, node *corev1.Node, err error, unitName, reason, msg string) {
	if recorder != nil && node != nil {
		if err == nil {
			recorder.Event(node, corev1.EventTypeNormal, reason, fmt.Sprintf("successfully %s unit %q", msg, unitName))
		} else {
			recorder.Event(node, corev1.EventTypeWarning, reason, fmt.Sprintf("failed to %s unit %q %v", msg, unitName, err))
		}
	}
}
