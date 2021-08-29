package storage

import (
	"fmt"
	"sort"
)

type walkFuncs struct {
	dir     func(path []string, dir *Dir) error
	file    func(path []string, file *File) error
	complex func(path []string, cf *ComplexFile) error
}

func (s *Storage) walk(path []string, dir *Dir, wf walkFuncs) error {
	if dir == nil {
		return fmt.Errorf("walk: dir is nil")
	}
	dir.mu.Lock()
	defer dir.mu.Unlock()
	// go through the entries in sorted order for predictable reports
	dirNames := make([]string, 0, len(dir.Entries))
	for name := range dir.Entries {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)
	for _, name := range dirNames {
		de := dir.Entries[name]
		curPath := append(path, name)
		switch de.Kind {
		case entryDir:
			if de.dir == nil {
				ded, err := DirLoad(dir.backing, de.Hash)
				if err != nil {
					return err
				}
				de.dir = ded
			}
			if wf.dir != nil {
				err := wf.dir(curPath, de.dir)
				if err != nil {
					return err
				}
			}
			err := s.walk(curPath, de.dir, wf)
			if err != nil {
				return err
			}
		case entryFile:
			if wf.file != nil {
				if de.file == nil {
					def, err := FileLoad(dir.backing, de.Hash)
					if err != nil {
						return err
					}
					de.file = def
				}
				err := wf.file(curPath, de.file)
				if err != nil {
					return err
				}
			}
		case entryComplex:
			if wf.complex != nil {
				if de.complex == nil {
					dec, err := ComplexLoad(dir.backing, de.Hash)
					if err != nil {
						return err
					}
					de.complex = dec
				}
				err := wf.complex(curPath, de.complex)
				if err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("walk: unknown directory entry, path = %v, kind = %v", curPath, de.Kind)
		}
	}

	return nil
}
