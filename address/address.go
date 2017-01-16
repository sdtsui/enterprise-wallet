package address

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/FactomProject/btcutil/base58"
	"github.com/FactomProject/factom"
)

var _ = fmt.Sprintf("")

const MaxNameLength int = 20

// Name/Address pair
type AddressNamePair struct {
	Name    string // Length maxNameLength Characters
	Address string
	Seeded  bool // Derived from seeed

	// Not Marshaled
	Balance float64 // Unused except for JSON return
}

func NewAddress(name string, address string) (*AddressNamePair, error) {
	if len(name) > MaxNameLength {
		return nil, fmt.Errorf("Name must be max %d characters", MaxNameLength)
	}

	if !factom.IsValidAddress(address) {
		return nil, errors.New("Address is invalid")
	}

	add := new(AddressNamePair)

	//var n [maxNameLength]byte
	//copy(n[:maxNameLength], name)

	add.Name = name
	add.Address = address
	add.Seeded = false

	return add, nil
}

// If addresses derives from the seed
func NewSeededAddress(name string, address string) (*AddressNamePair, error) {
	add, err := NewAddress(name, address)
	if err != nil {
		return nil, err
	}

	add.Seeded = true
	return add, nil
}

func (anp *AddressNamePair) ChangeName(name string) error {
	if len(name) > MaxNameLength {
		return fmt.Errorf("Name too long, must be less than %d characters", MaxNameLength)
	}
	anp.Name = name
	return nil
}

func (anp *AddressNamePair) IsSimilarTo(b *AddressNamePair) bool {
	if strings.Compare(anp.Address, b.Address) != 0 {
		return false
	}

	return true
}

func (anp *AddressNamePair) IsSameAs(b *AddressNamePair) bool {
	if !anp.IsSimilarTo(b) {
		return false
	}

	if strings.Compare(anp.Name, b.Name) != 0 {
		return false
	}

	return true
}

func (anp *AddressNamePair) MarshalBinary() (data []byte, err error) {
	buf := new(bytes.Buffer)

	var n [MaxNameLength]byte
	copy(n[:MaxNameLength], anp.Name)
	buf.Write(n[:MaxNameLength]) // 0:20

	add := base58.Decode(anp.Address)
	var a [38]byte
	copy(a[:38], add[:])
	buf.Write(a[:38]) // 20:58

	var b []byte
	b = strconv.AppendBool(b, anp.Seeded)
	if anp.Seeded {
		b = append(b, 0x00)
	}
	buf.Write(b) // 58:63

	return buf.Next(buf.Len()), nil
}

func (anp *AddressNamePair) UnmarshalBinaryData(data []byte) (newData []byte, err error) {
	newData = data

	nameData := bytes.Trim(newData[:MaxNameLength], "\x00")
	anp.Name = fmt.Sprintf("%s", nameData)
	newData = newData[MaxNameLength:]

	anp.Address = base58.Encode(newData[:38])
	newData = newData[38:]

	booldata := newData[:5]
	if booldata[4] == 0x00 {
		booldata = booldata[:4]
	}
	b, err := strconv.ParseBool(string(booldata))
	if err != nil {
		return data, err
	}
	anp.Seeded = b
	newData = newData[5:]

	return
}

func (anp *AddressNamePair) UnmarshalBinary(data []byte) (err error) {
	_, err = anp.UnmarshalBinaryData(data)
	return
}

//Address List
type AddressList struct {
	Length uint64
	List   []AddressNamePair
}

func NewAddressList() *AddressList {
	addList := new(AddressList)
	addList.Length = 0

	return addList
}

// Searches for Address
func (addList *AddressList) Get(address string) (*AddressNamePair, int) {
	if !factom.IsValidAddress(address) {
		return nil, -1
	}

	for i, ianp := range addList.List {
		if strings.Compare(ianp.Address, address) == 0 {
			return &ianp, i
		}
	}
	return nil, -1
}

func (addList *AddressList) AddANP(anp *AddressNamePair) error {
	if len(anp.Name) == 0 || !factom.IsValidAddress(anp.Address) {
		return errors.New("Nil AddressNamePair")
	}

	_, i := addList.Get(anp.Address)
	if i == -1 {
		addList.List = append(addList.List, *anp)
		addList.Length++
		return nil
	}

	// Duplicate Found
	return errors.New("Address or Name already exists")

}

func (addList *AddressList) AddSeeded(name string, address string) (*AddressNamePair, error) {
	anp, err := NewSeededAddress(name, address)
	if err != nil {
		return nil, err
	}
	return addList.add(anp)
}

func (addList *AddressList) Add(name string, address string) (*AddressNamePair, error) {
	anp, err := NewAddress(name, address)
	if err != nil {
		return nil, err
	}
	return addList.add(anp)
}

func (addList *AddressList) add(anp *AddressNamePair) (*AddressNamePair, error) {
	// We check for valid factom address higher up, this is just a basic check
	if len(anp.Name) == 0 {
		return nil, errors.New("Nil AddressNamePair")
	}

	_, i := addList.Get(anp.Address)
	if i == -1 {
		addList.List = append(addList.List, *anp)
		addList.Length++
		return anp, nil
	}

	// Duplicate Found
	return nil, errors.New("Address already exists")

}

func (addList *AddressList) Remove(removeAdd string) error {
	_, i := addList.Get(removeAdd)
	if i == -1 {
		return errors.New("Not found")
	}
	addList.Length--
	addList.List = append(addList.List[:i], addList.List[i+1:]...)

	return nil
}

func (addList *AddressList) ResetSeeded() {
	for i := range addList.List {
		addList.List[i].Seeded = false
	}
}

func (addList *AddressList) MarshalBinary() (data []byte, err error) {
	buf := new(bytes.Buffer)
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], addList.Length)
	buf.Write(number[:])

	for _, anp := range addList.List {
		anpData, err := anp.MarshalBinary()
		if err != nil {
			return nil, err
		}
		buf.Write(anpData)
	}

	return buf.Next(buf.Len()), err
}

func (addList *AddressList) UnmarshalBinaryData(data []byte) (newData []byte, err error) {
	newData = data

	addList.Length = binary.BigEndian.Uint64(data[:8])
	newData = newData[8:]

	var i uint64 = 0
	for i < addList.Length {
		anp := new(AddressNamePair)
		newData, err = anp.UnmarshalBinaryData(newData)
		addList.List = append(addList.List, *anp)
		i++
	}

	return
}

func (addList *AddressList) UnmarshalBinary(data []byte) (err error) {
	_, err = addList.UnmarshalBinaryData(data)
	return
}

func (addList *AddressList) IsSameAs(b *AddressList) bool {
	if addList.Length != b.Length {
		return false
	}

	for _, anp := range addList.List {
		if inap, i := b.Get(anp.Address); i == -1 || !anp.IsSameAs(inap) {
			return false
		}
	}

	return true
}
