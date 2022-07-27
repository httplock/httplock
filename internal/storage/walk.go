package storage

import (
	"fmt"
	"sort"
)

type WalkFuncs struct {
	Dir     func(path []string, dir *Dir) error
	File    func(path []string, file *File) error
	Complex func(path []string, cf *ComplexFile) error
}

func (s *Storage) Walk(path []string, dir *Dir, wf WalkFuncs) error {
	if dir == nil {
		return fmt.Errorf("walk: dir is nil")
	}
	dir.mu.Lock()
	defer dir.mu.Unlock()
	if wf.Dir != nil {
		err := wf.Dir(path, dir)
		if err != nil {
			return err
		}
	}
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
			err := s.Walk(curPath, de.dir, wf)
			if err != nil {
				return err
			}
		case entryFile:
			if wf.File != nil {
				if de.file == nil {
					def, err := FileLoad(dir.backing, de.Hash)
					if err != nil {
						return err
					}
					de.file = def
				}
				err := wf.File(curPath, de.file)
				if err != nil {
					return err
				}
			}
		case entryComplex:
			if wf.Complex != nil {
				if de.complex == nil {
					dec, err := ComplexLoad(dir.backing, de.Hash)
					if err != nil {
						return err
					}
					de.complex = dec
				}
				err := wf.Complex(curPath, de.complex)
				if err != nil {
					return err
				}
				if wf.File != nil {
					for _, entry := range de.complex.Entries {
						cef, err := FileLoad(dir.backing, entry.Hash)
						if err != nil {
							return err
						}
						err = wf.File(curPath, cef)
						if err != nil {
							return err
						}
					}
				}
			}
		default:
			return fmt.Errorf("walk: unknown directory entry, path = %v, kind = %v", curPath, de.Kind)
		}
	}

	return nil
}
