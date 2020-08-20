package in_toto

import (
	"fmt"
	"path/filepath"
)

/*
Set represents a data structure for set operations. See `NewSet` for how to
create a Set, and available Set receivers for useful set operations.

Under the hood Set aliases map[string]struct{}, where the map keys are the set
elements and the map values are a memory-efficient way of storing the keys.
*/
type Set map[string]struct{}

/*
NewSet creates a new Set, assigns it the optionally passed variadic string
elements, and returns it.
*/
func NewSet(elems ...string) Set {
	var s Set
	s = make(map[string]struct{})
	for _, elem := range elems {
		s.Add(elem)
	}
	return s
}

/*
Has returns True if the passed string is member of the set on which it was
called and False otherwise.
*/
func (s Set) Has(elem string) bool {
	_, ok := s[elem]
	return ok
}

/*
Add adds the passed string to the set on which it was called, if the string is
not a member of the set.
*/
func (s Set) Add(elem string) {
	s[elem] = struct{}{}
}

/*
Remove removes the passed string from the set on which was is called, if the
string is a member of the set.
*/
func (s Set) Remove(elem string) {
	delete(s, elem)
}

/*
Intersection creates and returns a new Set with the elements of the set on
which it was called that are also in the passed set.
*/
func (s Set) Intersection(s2 Set) Set {
	res := NewSet()
	for elem := range s {
		if s2.Has(elem) == false {
			continue
		}
		res.Add(elem)
	}
	return res
}

/*
Difference creates and returns a new Set with the elements of the set on
which it was called that are not in the passed set.
*/
func (s Set) Difference(s2 Set) Set {
	res := NewSet()
	for elem := range s {
		if s2.Has(elem) {
			continue
		}
		res.Add(elem)
	}
	return res
}

/*
Filter creates and returns a new Set with the elements of the set on which it
was called that match the passed pattern. A matching error is treated like a
non-match plus a warning is printed.
*/
func (s Set) Filter(pattern string) Set {
	res := NewSet()
	for elem := range s {
		matched, err := filepath.Match(pattern, elem)
		if err != nil {
			fmt.Printf("WARNING: %s, pattern was '%s'\n", err, pattern)
			continue
		}
		if !matched {
			continue
		}
		res.Add(elem)
	}
	return res
}

/*
Slice creates and returns an unordered string slice with the elements of the
set on which it was called.
*/
func (s Set) Slice() []string {
	var res []string
	res = make([]string, 0, len(s))
	for elem := range s {
		res = append(res, elem)
	}
	return res
}

/*
InterfaceKeyStrings returns string keys of passed interface{} map in an
unordered string slice.
*/
func InterfaceKeyStrings(m map[string]interface{}) []string {
	res := make([]string, len(m))
	i := 0
	for k := range m {
		res[i] = k
		i++
	}
	return res
}

/*
subsetCheck checks if all strings in a slice of strings
can be found in a superset slice of strings.
*/
func subsetCheck(subset []string, superset []string) bool {
	// TODO: This function might be better as addition
	// to our Set interface.
	// We use a Go label here to break out to the outer loop
OUTER:
	for _, sub := range subset {
		for _, super := range superset {
			if sub == super {
				continue OUTER
			}
		}
		// If we cannot find a substring from subset in the superset
		// we return false. In terms of keyIdHashAlgorithms for example
		// this would mean, that we have an unsupported hash algorithm
		// in our keyIdHashAlgorithm slice.
		return false
	}
	// return true if all substrings can be found in the superset
	return true
}
