package bptree

import (
)

func (self *BpTree) Remove(key []byte, where func([]byte) bool) (err error) {
	a, err := self.remove(self.meta.root, 0, key, where)
	if err != nil {
		return err
	}
	if a == 0 {
		a, err = self.newLeaf()
		if err != nil {
			return err
		}
	}
	self.meta.root = a
	return self.writeMeta()
}

func (self *BpTree) remove(n, sibling uint64, key []byte, where func([]byte) bool) (a uint64, err error) {
	var flags flag
	err = self.bf.Do(n, 1, func(bytes []byte) error {
		flags = flag(bytes[0])
		return nil
	})
	if err != nil {
		return 0, err
	}
	if flags & INTERNAL != 0 {
		return self.internalRemove(n, sibling, key, where)
	} else if flags & LEAF != 0 {
		return self.leafRemove(n, sibling, key, where)
	} else {
		return 0, Errorf("Unknown block type")
	}
}

func (self *BpTree) internalRemove(n, sibling uint64, key []byte, where func([]byte) bool) (a uint64, err error) {
	var i int
	var kid uint64
	err = self.doInternal(n, func(n *internal) error {
		var has bool
		i, has = find(int(n.meta.keyCount), n.keys, key)
		if !has && i > 0 {
			// if it doesn't have it and the index > 0 then we have the
			// next block so we have to subtract one from the index.
			i--
		}
		kid = n.ptrs[i]
		if i + 1 < int(n.meta.keyCount) {
			sibling = n.ptrs[i+1]
		} else if sibling != 0 {
			return self.doInternal(sibling, func(m *internal) error {
				sibling = m.ptrs[0]
				return nil
			})
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	kid, err = self.remove(kid, sibling, key, where)
	if err != nil {
		return 0, err
	}
	if kid == 0 {
		err = self.doInternal(n, func(n *internal) error {
			return n.delItemAt(i)
		})
		if err != nil {
			return 0, err
		}
	} else {
		err = self.doInternal(n, func(n *internal) error {
			n.ptrs[i] = kid
			return self.firstKey(kid, func(kid_key []byte) error {
				copy(n.keys[i], kid_key)
				return nil
			})
		})
		if err != nil {
			return 0, err
		}
	}
	var keyCount uint16
	err = self.doInternal(n, func(n *internal) error {
		keyCount = n.meta.keyCount
		return nil
	})
	if err != nil {
		return 0, err
	}
	if keyCount == 0 {
		return 0, nil
	}
	return n, nil
}

func (self *BpTree) leafRemove(a, sibling uint64, key []byte, where func([]byte) bool) (b uint64, err error) {
	var i int
	err = self.doLeaf(a, func(n *leaf) error {
		var has bool
		i, has = find(int(n.meta.keyCount), n.keys, key)
		if !has {
			return Errorf("key was not in tree")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	next, err := self.forwardFrom(a, i, key)
	if err != nil {
		return 0, err
	}
	b = a
	for a, i, err, next = next(); next != nil; a, i, err, next = next() {
		err = self.doLeaf(a, func(n *leaf) error {
			var remove bool = false
			err = n.doValueAt(self.bf, i, func(value []byte) error {
				remove = where(value)
				return nil
			})
			if err != nil {
				return err
			}
			if remove {
				err = n.delItemAt(i)
				if err != nil {
					return err
				}
			}
			if int(n.meta.keyCount) <= 0 {
				err = delListNode(self.bf, a)
				if err != nil {
					return err
				}
				if n.meta.next == 0 {
					b = 0
				} else if sibling == 0 {
					b = 0
				} else if n.meta.next != sibling {
					b = n.meta.next
				} else {
					b = 0
				}
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	if err != nil {
		return 0, err
	}
	return b, nil
}
