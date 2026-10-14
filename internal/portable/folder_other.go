//go:build !windows

package portable

import "errors"

func ChooseDataDir() (string, error) { return "", errors.New("select a data path with -data") }
