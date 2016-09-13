// jsonschema2go is the command invoked by go generate in order to generate the go client library.
package main

import (
	"bufio"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"golang.org/x/tools/imports"

	docopt "github.com/docopt/docopt-go"
	"github.com/taskcluster/jsonschema2go"
)

func readStringStrip(reader *bufio.Reader, delimeter byte) (string, error) {
	token, err := reader.ReadString(delimeter)
	if err != nil {
		return "", err
	}
	// strip delimeter from end of string
	if token != "" {
		token = token[:len(token)-1]
	}
	return token, nil
}

func parseStandardIn() ([]string, error) {
	results := []string{}
	reader := bufio.NewReader(os.Stdin)
	for {
		url, err := readStringStrip(reader, '\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		results = append(results, url)
	}
	return results, nil
}

var (
	version = "jsonschema2go 2.0.0"
	usage   = `
jsonschema2go
jsonschema2go generates go source code from json schema inputs. Specifically,
it returns a []byte of source code that can be written to a file, for all
objects found in the provided json schemas, plus any schemas that they
reference. It will automatically download json schema definitions referred to
in the provided schemas, if there are cross references to external json schemas
hosted on an available url (i.e. $ref property of json schema). You can either
pass urls via standard in (one per line) or provide a space-separated list of
urls via the --in command line argument.

The go type names will be "normalised" from the json subschema Title element.

Examples:
  cat urls.txt | jsonschema2go --out cli.go -o main
  jsonschema2go --in "https://.../url1 file:///Users/pmoore/myschema.yml" --build '!windows' -- monkey

Usage:
  jsonschema2go [--in INPUT-URLS] [--out OUTPUT-FILE] [--build BUILD-DIRECTIVES] [--] GO-PACKAGE-NAME
  jsonschema2go -h|--help
  jsonschema2go --version

Options:
--in INPUT-URLS            A list of URLs to input. If not provided, the urls
                           will be read from standard in.
--out OUTPUT-FILE          The file to write the generated code to. The file
                           will be overwritten, if it already exists, or
                           created if needed. If not specified, generated
                           code will be written to standard out.
--build BUILD-DIRECTIVES   If build directives are specified, the generated
                           code will begin with the line:
                           '// +build <BUILD-DIRECTIVES>'
-h --help                  Display this help text.
--version                  Display the version information of jsonschema2go.
`
)

func main() {
	// Parse the docopt string and exit on any error or help message.
	arguments, err := docopt.Parse(usage, nil, true, version, false, true)
	if err != nil {
		log.Fatalf("jsonschema2go: Could not parse command line arguments: '%#v'", err)
	}
	urls := []string{}
	if in := arguments["INPUT-URLS"]; in != nil {
		urls = strings.Split(in.(string), " ")
	} else {
		urls, err = parseStandardIn()
		if err != nil {
			log.Fatalf("jsonschema2go: Could not read input URLs from standard in: '%#v'", err)
		}
	}
	job := &jsonschema2go.Job{
		Package:     arguments["GO-PACKAGE-NAME"].(string),
		ExportTypes: true,
		URLs:        urls,
	}
	result, err := job.Execute()
	if err != nil {
		log.Fatalf("jsonschema2go: Could not generate source code: '%#v'", err)
	}
	if directives := arguments["BUILD-DIRECTIVES"]; directives != nil {
		result.SourceCode = append([]byte("// +build "+directives.(string)+"\n"), result.SourceCode...)
	}
	if out := arguments["OUTPUT-FILE"]; out != nil {
		err = formatSourceAndSave(out.(string), result.SourceCode)
		if err != nil {
			log.Fatalf("jsonschema2go: Could not create file '%v'", out)
		}
	} else {
		fmt.Println(string(result.SourceCode))
	}
}

func formatSourceAndSave(sourceFile string, sourceCode []byte) error {
	// first run goimports to clean up unused imports
	fixedImports, err := imports.Process(sourceFile, sourceCode, nil)
	var formattedContent []byte
	// only perform general format, if that worked...
	if err == nil {
		// now run a standard system format
		formattedContent, err = format.Source(fixedImports)
	}
	// in case of formatting failure from either of the above formatting
	// steps, let's keep the unformatted version so we can troubleshoot
	// more easily...
	if err != nil {
		// no need to handle error as we exit below anyway
		_ = ioutil.WriteFile(sourceFile, sourceCode, 0644)
		return err
	}
	return ioutil.WriteFile(sourceFile, formattedContent, 0644)
}
