// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package run

import (
	"fmt"
	"os"
	"strings"

	"github.com/kardianos/govendor/context"
	"github.com/kardianos/govendor/internal/pathos"
	"github.com/kardianos/govendor/pkgspec"
)

var (
	outside = []context.Status{
		{Location: context.LocationExternal},
		{Presence: context.PresenceMissing},
	}
	normal = []context.Status{
		{Location: context.LocationExternal},
		{Location: context.LocationVendor},
		{Location: context.LocationLocal},
		{Location: context.LocationNotFound},
	}
	all = []context.Status{
		{Location: context.LocationStandard},
		{Location: context.LocationExternal},
		{Location: context.LocationVendor},
		{Location: context.LocationLocal},
		{Location: context.LocationNotFound},
	}
)

func statusGroupFromList(list []context.Status, and, not bool) context.StatusGroup {
	sg := context.StatusGroup{
		Not: not,
		And: and,
	}
	for _, s := range list {
		sg.Status = append(sg.Status, s)
	}
	return sg
}

const notOp = "^"

func parseStatusGroup(statusString string) (sg context.StatusGroup, err error) {
	ss := strings.Split(statusString, ",")
	sg.And = true
	for _, s := range ss {
		st := context.Status{}
		if strings.HasPrefix(s, notOp) {
			st.Not = true
			s = strings.TrimPrefix(s, notOp)
		}
		var list []context.Status
		switch {
		case strings.HasPrefix("external", s):
			st.Location = context.LocationExternal
		case strings.HasPrefix("vendor", s):
			st.Location = context.LocationVendor
		case strings.HasPrefix("unused", s):
			st.Presence = context.PresenceUnsued
		case strings.HasPrefix("missing", s):
			st.Presence = context.PresenceMissing
		case strings.HasPrefix("local", s):
			st.Location = context.LocationLocal
		case strings.HasPrefix("program", s):
			st.Type = context.TypeProgram
		case strings.HasPrefix("std", s):
			st.Location = context.LocationStandard
		case strings.HasPrefix("standard", s):
			st.Location = context.LocationStandard
		case strings.HasPrefix("all", s):
			list = all
		case strings.HasPrefix("normal", s):
			list = normal
		case strings.HasPrefix("outside", s):
			list = outside
		default:
			err = fmt.Errorf("unknown status %q", s)
			return
		}
		if len(list) == 0 {
			sg.Status = append(sg.Status, st)
		} else {
			sg.Group = append(sg.Group, statusGroupFromList(list, false, st.Not))
		}
	}
	return
}

type filterImport struct {
	Pkg   *pkgspec.Pkg
	Added bool // Used to prevent imports from begin added twice.
}

func (f *filterImport) String() string {
	return f.Pkg.String()
}

type filter struct {
	Status context.StatusGroup
	Import []*filterImport
}

func (f filter) String() string {
	return fmt.Sprintf("status %q, import: %q", f.Status, f.Import)
}

func (f filter) HasStatus(item context.StatusItem) bool {
	return item.Status.MatchGroup(f.Status)
}
func (f filter) FindImport(item context.StatusItem) *filterImport {
	for _, imp := range f.Import {
		if imp.Pkg.Path == item.Local || imp.Pkg.Path == item.Canonical {
			imp.Added = true
			return imp
		}
		if imp.Pkg.MatchTree {
			if strings.HasPrefix(item.Local, imp.Pkg.Path) || strings.HasPrefix(item.Canonical, imp.Pkg.Path) {
				imp.Added = true
				return imp
			}
		}
	}
	return nil
}

func currentGoPath(ctx *context.Context) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	wdpath := pathos.FileTrimPrefix(wd, ctx.RootGopath)
	wdpath = pathos.SlashToFilepath(wdpath)
	wdpath = strings.Trim(wdpath, "/")
	return wdpath, nil
}

func parseFilter(currentGoPath string, args []string) (filter, error) {
	f := filter{
		Import: make([]*filterImport, 0, len(args)),
	}
	for _, a := range args {
		if len(a) == 0 {
			continue
		}
		// Check if item is a status.
		if a[0] == '+' {
			sg, err := parseStatusGroup(a[1:])
			if err != nil {
				return f, err
			}
			f.Status.Group = append(f.Status.Group, sg)
		} else {
			pkg, err := pkgspec.Parse(currentGoPath, a)
			if err != nil {
				return f, err
			}
			f.Import = append(f.Import, &filterImport{Pkg: pkg})
		}
	}
	return f, nil
}

func insertListToAllNot(sg *context.StatusGroup, list []context.Status) {
	if len(sg.Group) == 0 {
		allStatusNot := true
		for _, s := range sg.Status {
			if s.Not == false {
				allStatusNot = false
				break
			}
		}
		if allStatusNot {
			sg.Group = append(sg.Group, statusGroupFromList(list, false, false))
		}
	}
	for i := range sg.Group {
		insertListToAllNot(&sg.Group[i], list)
	}
}
