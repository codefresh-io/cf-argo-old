package helpers

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/yargevad/filepathx"
)

func CopyDir(source, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		var relPath string = strings.Replace(path, source, "", 1)
		if relPath == "" {
			return nil
		}
		if info.IsDir() {
			return os.Mkdir(filepath.Join(destination, relPath), info.Mode())
		} else {
			var data, err1 = ioutil.ReadFile(filepath.Join(source, relPath))
			if err1 != nil {
				return err1
			}
			return ioutil.WriteFile(filepath.Join(destination, relPath), data, info.Mode())
		}
	})
}

func RenderDirRecurse(pattern string, values interface{}) error {
	matches, err := filepathx.Glob(pattern)
	if err != nil {
		return err
	}

	for _, match := range matches {
		if strings.HasSuffix(match, "secret.yaml") { // TODO: make prettier
			continue
		}

		tpl, err := template.ParseFiles(match)
		if err != nil {
			return err
		}

		fw, err := os.OpenFile(match, os.O_WRONLY|os.O_TRUNC, 0)
		if err != nil {
			return err
		}

		err = tpl.Execute(fw, values)
		fw.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
