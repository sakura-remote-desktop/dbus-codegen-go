package printer

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"strings"

	"github.com/amenzhinsky/godbus-codegen/token"
)

type buffer struct {
	buf bytes.Buffer
}

func (b *buffer) writef(format string, v ...interface{}) (int, error) {
	return fmt.Fprintf(&b.buf, format, v...)
}

func (b *buffer) writeln(s ...string) {
	for i := 0; i < len(s); i++ {
		b.buf.WriteString(s[i])
	}
	b.buf.WriteByte('\n')
}

func (b *buffer) bytes() []byte {
	return b.buf.Bytes()
}

func Print(out io.Writer, pkgName string, ifaces []*token.Interface) error {
	buf := &buffer{}
	if err := writeHeader(buf, pkgName, ifaces); err != nil {
		return err
	}
	signals := map[string][]*token.Signal{}
	for _, iface := range ifaces {
		if err := writeIface(buf, iface, signals); err != nil {
			return err
		}
	}
	if len(signals) > 0 {
		if err := writeSignalFuncs(buf, signals); err != nil {
			return err
		}
	}

	// gofmt code
	b, err := format.Source(buf.bytes())
	if err != nil {
		return err
	}
	_, err = out.Write(b)
	return err
}

func writeHeader(buf *buffer, pkgName string, ifaces []*token.Interface) error {
	buf.writeln("// Generated by dbusgen, don't edit!")
	buf.writeln("//")
	for i, iface := range ifaces {
		if i != 0 {
			buf.writeln("//")
		}
		buf.writeln("// ", iface.Name)
		if len(iface.Methods) != 0 {
			buf.writeln("//   Methods")
			for _, method := range iface.Methods {
				buf.writeln("//     ", method.Name)
			}
		}
		if len(iface.Properties) != 0 {
			buf.writeln("//   Properties")
			for _, prop := range iface.Properties {
				buf.writeln("//     ", prop.Name)
			}
		}
		if len(iface.Signals) != 0 {
			buf.writeln("//   Signals")
			for _, sig := range iface.Signals {
				buf.writeln("//     ", sig.Name)
			}
		}
	}
	buf.writeln("package ", pkgName)
	buf.writeln(`import "github.com/godbus/dbus"`)
	return nil
}

func writeIface(buf *buffer, iface *token.Interface, signals map[string][]*token.Signal) error {
	buf.writef(`// %s returns %s DBus interface implementation.
func New%s(conn *dbus.Conn, dest string, path dbus.ObjectPath) *%s {
	return &%s{conn.Object(dest, path)}
}
`, iface.Type, iface.Name, iface.Type, iface.Type, iface.Type)
	buf.writef(`// %s implements %s DBus interface.
type %s struct {
	object dbus.BusObject
}
`,
		iface.Type, iface.Name,
		iface.Type,
	)

	for _, method := range iface.Methods {
		buf.writef(`// %s calls %s.%s method.
func(o *%s) %s(%s) (%serr error) {
	err = o.object.Call("%s", 0, %s).Store(%s)
	return
}
`,
			iface.Type, iface.Name, method.Name,
			iface.Type, method.Type, joinArgs(method.In, ','), joinArgs(method.Out, ','),
			iface.Name+"."+method.Type, joinArgNames(method.In), joinStoreArgs(method.Out),
		)
	}

	for _, prop := range iface.Properties {
		if prop.Read {
			buf.writef(`// %s gets %s.%s property.
func(o *%s) %s() (%s %s, err error) {
	o.object.Call("org.freedesktop.DBus.Properties.Get", 0, "%s", "%s").Store(&%s)
	return
}
`,
				prop.Type, iface.Name, prop.Name,
				iface.Type, prop.Type, prop.Arg.Name, prop.Arg.Type,
				iface.Name, prop.Name, prop.Arg.Name,
			)
		}
	}

	if len(iface.Signals) > 0 {
		signals[iface.Name] = iface.Signals
	}
	for _, sig := range iface.Signals {
		buf.writef(`// %s represents %s.%s signal.
type %s struct {
	sender string
	path   dbus.ObjectPath
	body   %sBody
}

type %sBody struct {
	%s
}

// Name returns the signal's name.
func (s *%s) Name() string {
	return "%s"
}

// Interface returns the signal's interface.
func (s *%s) Interface() string {
	return "%s"
}

// Sender returns the signal's sender unique name.
func (s *%s) Sender() string {
	return s.sender
}

// Path returns path that emitted the signal. 
func (s *%s) Path() dbus.ObjectPath {
	return s.path
}

// Body returns the signals' payload.
func (s *%s) Body() %sBody {
	return s.body
}
`,
			sig.Type, sig.Name, iface.Name,
			sig.Type, sig.Type, sig.Type, joinArgs(sig.Args, ';'),
			sig.Type, sig.Name, sig.Type, iface.Name, sig.Type, sig.Type,
			sig.Type, sig.Type,
		)
	}
	return nil
}

func writeSignalFuncs(buf *buffer, signals map[string][]*token.Signal) error {
	buf.writef(`// Signal is a common interface for all signals.
type Signal interface {
	Name() string
	Interface() string
	Sender() string
	Path() dbus.ObjectPath
}
`)
	buf.writef(`// LookupSignal converts the given raw DBus signal into typed one.
func LookupSignal(signal *dbus.Signal) Signal {
	switch signal.Name {
`)
	for iface, sigs := range signals {
		for _, sig := range sigs {
			buf.writef(`	case "%s.%s":
		return &%s{
			sender: signal.Sender,
			path:   signal.Path,
			body:   %sBody{
`, iface, sig.Name, sig.Type, sig.Type)
			for i, arg := range sig.Args {
				buf.writef("				%s: signal.Body[%d].(%s),\n", arg.Name, i, arg.Type)
			}
			buf.writeln("			},")
			buf.writeln("		}")
		}
	}
	buf.writef(`	default:
		return nil
	}
}
`)
	return nil
}

func joinStoreArgs(args []*token.Arg) string {
	var buf strings.Builder
	for i := range args {
		if i != 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('&')
		buf.WriteString(args[i].Name)
	}
	return buf.String()
}

func joinArgs(args []*token.Arg, separator byte) string {
	var buf strings.Builder
	for i := range args {
		buf.WriteString(args[i].Name)
		buf.WriteByte(' ')
		buf.WriteString(args[i].Type)
		buf.WriteByte(separator)
	}
	return buf.String()
}

func joinArgNames(args []*token.Arg) string {
	var buf strings.Builder
	for i := range args {
		if i != 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(args[i].Name)
	}
	return buf.String()
}
