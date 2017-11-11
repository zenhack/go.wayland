package main

import (
	"encoding/xml"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"unicode"
)

var reservedWords = map[string]struct{}{
	"interface": {},
	"struct":    {},
}

// Types for unmarshalling the xml file:

type Protocol struct {
	Name       WlName      `xml:"name,attr"`
	Copyright  string      `xml:"copyright"`
	Interfaces []Interface `xml:"interface"`
}

type Interface struct {
	Name        WlName    `xml:"name,attr"`
	Description string    `xml:"description"`
	Requests    []Request `xml:"request"`
	Events      []Event   `xml:"event"`
}

type Request struct {
	Name        WlName `xml:"name,attr"`
	Description string `xml:"description"`
	Args        []Arg  `xml:"arg"`
}

type Event struct {
	Name        WlName `xml:"name,attr"`
	Description string `xml:"description"`
	Args        []Arg  `xml:"arg"`
}

type Arg struct {
	Name      WlName `xml:"name,attr"`
	Type      WlType `xml:"type,attr"`
	Summary   string `xml:"summary,attr"`
	Interface WlName `xml:"interface,attr"`
}

type Enum struct {
	Description string  `xml:"description"`
	Entries     []Entry `xml:"entry"`
}

type Entry struct {
	Name    WlName `xml:"name,attr"`
	Value   uint64 `xml:"value,attr"`
	Summary string `xml:"summary,attr"`
}

// A wrapper for wayland basic types
type WlType string

func (t WlType) GoName() string {
	switch t {
	case "fd":
		return "int"
	case "object":
		return "ObjectId"
	case "uint":
		return "uint32"
	case "int":
		return "int32"
	case "fixed":
		return "Fixed"
	case "array":
		// TODO: the spec doesn't say anything about the element type.
		return "[]byte"
	default:
		return string(t)
	}
}

// A wrapper for wayland identifiers
type WlName string

// Split the identifier on underscores, and remove a leading "wl", if any.
func (n WlName) parts() []string {
	ret := strings.Split(string(n), "_")
	if ret[0] == "wl" {
		ret = ret[1:]
	}
	return ret
}

// capitalize the first letter of each string in the slice.
// The elements must be ascii.
func titleCase(parts []string) {
	for i := range parts {
		word := []byte(parts[i])
		if len(word) == 0 {
			continue
		}
		// Everything is ascii, so we can assume each rune is one byte:
		word[0] = byte(unicode.ToUpper(rune(word[0])))
		parts[i] = string(word)
	}
}

// Convert the identifier to an exported idiomatic go variable name.
func (n WlName) Exported() string {
	parts := n.parts()
	titleCase(parts)
	return strings.Join(parts, "")
}

// Convert the identifier to a private/local idiomatic go variable name.
func (n WlName) Local() string {
	parts := n.parts()
	titleCase(parts[1:])
	ret := strings.Join(parts, "")
	_, ok := reservedWords[ret]
	if ok {
		ret += "_"
	}
	return ret
}

// Helper for simple error handling
func chkfatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	tpls := template.Must(template.ParseGlob("./internal/gen/templates/*"))
	proto := Protocol{}
	buf, err := ioutil.ReadFile("wayland.xml")
	chkfatal(err)
	err = xml.Unmarshal(buf, &proto)
	chkfatal(err)
	file, err := os.Create("gen.go")
	chkfatal(err)
	defer file.Close()
	chkfatal(tpls.ExecuteTemplate(file, "protocol", proto))
	chkfatal(exec.Command("gofmt", "-s", "-w", "gen.go").Run())
}
