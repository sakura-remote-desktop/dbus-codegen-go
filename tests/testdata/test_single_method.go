package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sakura-remote-desktop/godbus/v5"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	o := NewOrg_Freedesktop_DBus(
		conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus"),
	)
	_, err = o.GetId(context.Background())
	return err
}
