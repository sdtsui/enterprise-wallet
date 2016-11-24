package wallet

/*
 * Manages all the addresses and 2 databases (Wallet DB and GUI DB)
 *
 */

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"strings"
	"sync"

	"github.com/FactomProject/M2WalletGUI/address"
	"github.com/FactomProject/M2WalletGUI/wallet/database"
	"github.com/FactomProject/factom"
	"github.com/FactomProject/factom/wallet"
	// "github.com/FactomProject/factom/wallet/wsapi"
	"encoding/json"
	"github.com/FactomProject/factomd/common/interfaces"
)

const (
	MAP int = iota
	LDB
	BOLT
)

var (
	GUI_DB    = MAP
	WALLET_DB = MAP
)

// Wallet interacting with LDB and factom/wallet
//   The LDB doesn't need to be updated often, so we save after every add and only
//   deal with cached version
type WalletDB struct {
	GUIlDB        database.IDatabase        // GUI DB
	guiWallet     *WalletStruct             // Cached version on GUI LDB
	Wallet        *wallet.Wallet            // Wallet from factom/wallet
	TransactionDB *wallet.TXDatabaseOverlay // Used to display transactions

	// List of transactions related to any address in address book
	cachedTransactions []interfaces.ITransaction
	cachedHeight       int64
}

// For now is same as New
func LoadWalletDB() (*WalletDB, error) {
	return NewWalletDB()
}

func NewWalletDB() (*WalletDB, error) {
	w := new(WalletDB)

	// TODO: Adjust this path
	var db database.IDatabase
	var err error
	switch GUI_DB { // Decides type of wallet DB
	case MAP:
		db, err = database.NewMapDB()
	case LDB:
		db, err = database.NewLevelDB(GetHomeDir() + "/.factom/m2/gui_wallet_ldb")
	case BOLT:
	}
	if err != nil {
		return nil, err
	}

	w.GUIlDB = db

	w.guiWallet = NewWallet()
	data, err := w.GUIlDB.Get([]byte("wallet"))
	if err == nil {
		err = w.guiWallet.UnmarshalBinary(data)
	}
	if err != nil {
		data, err := w.guiWallet.MarshalBinary()
		if err != nil {
			return nil, err
		}

		err = w.GUIlDB.Put([]byte("wallet"), data)
		if err != nil {
			return nil, err
		}
	}

	// TODO: Adjust this path
	var wal *wallet.Wallet
	switch GUI_DB { // Decides type of wallet DB
	case MAP:
		wal, err = wallet.NewMapDBWallet()
	case LDB:
		wal, err = wallet.NewOrOpenLevelDBWallet(GetHomeDir() + "/.factom/m2/gui_wallet_testing")
	case BOLT:
		wal, err = wallet.NewOrOpenBoltDBWallet(GetHomeDir() + "/.factom/m2/gui_wallet_testing.db")
	}
	if err != nil {
		return nil, err
	}
	w.Wallet = wal

	txdb, err := wallet.NewTXBoltDB(fmt.Sprint(GetHomeDir(), "/.factom/wallet/factoid_blocks.cache"))
	if err != nil {
		return nil, fmt.Errorf("Could not add transaction database to wallet:", err)
	} else {
		w.Wallet.AddTXDB(txdb)
	}

	w.TransactionDB = w.Wallet.TXDB()

	err = w.UpdateGUIDB()
	if err != nil {
		return nil, err
	}

	// go wsapi.Start(w.Wallet, fmt.Sprintf(":%d", 8089), *(factom.RpcConfig))

	return w, nil
}

func (w *WalletDB) GetGUIWalletJSON() ([]byte, error) {
	w.addBalancesToAddresses()
	return json.Marshal(w.guiWallet)
}

func (w *WalletDB) addBalancesToAddresses() {
	w.guiWallet.addBalancesToAddresses()
}

// Grabs the list of addresses from the walletDB and updates our GUI
// with any that are missing. All will be external
func (w *WalletDB) UpdateGUIDB() error {
	faAdds, ecAdds, err := w.Wallet.GetAllAddresses()
	if err != nil {
		return err
	}

	var names []string
	var addresses []string

	// Add addresses to GUI from cli
	for _, fa := range faAdds {
		_, list := w.GetGUIAddress(fa.String())
		if list == 0 {
			names = append(names, "FAImported-Undefined")
			addresses = append(addresses, fa.String())
		}
	}

	for _, ec := range ecAdds {
		_, list := w.GetGUIAddress(ec.String())
		if list == 0 {
			names = append(names, "ECImported-Undefined")
			addresses = append(addresses, ec.String())
		}
	}

	if len(names) > 0 {
		err = w.addBatchGUIAddresses(names, addresses)
		if err != nil {
			return err
		}
	}

	// Todo: Remove addresses that were deleted in cli?

	return w.Save()
}

func (w *WalletDB) Close() error {
	// Combine all close errors, as all need to get closed
	errCount := 0
	errString := ""

	err := w.Save()
	if err != nil {
		errCount++
		errString = errString + "; " + err.Error()
	}
	err = w.Wallet.Close()
	if err != nil {
		errCount++
		errString = errString + "; " + err.Error()
	}
	err = w.GUIlDB.Close()
	if err != nil {
		errCount++
		errString = errString + "; " + err.Error()
	}

	if errCount > 0 {
		return fmt.Errorf("Found %d errors: %s", errCount, errString)
	}
	return nil
}

func (w *WalletDB) Save() error {
	data, err := w.guiWallet.MarshalBinary()
	if err != nil {
		return err
	}

	err = w.GUIlDB.Put([]byte("wallet"), data)
	if err != nil {
		return err
	}

	return nil
}

func (w *WalletDB) GenerateFactoidAddress(name string) (*address.AddressNamePair, error) {
	address, err := w.Wallet.GenerateFCTAddress()

	if err != nil {
		return nil, err
	}

	anp, err := w.guiWallet.AddAddress(name, address.String(), 1)
	if err != nil {
		return nil, err
	}

	err = w.Save()
	if err != nil {
		return nil, err
	}
	return anp, nil
}

func (w *WalletDB) GetPrivateKey(address string) (secret string, err error) {
	if !factom.IsValidAddress(address) {
		return "", fmt.Errorf("Not a valid address")
	}

	if address[:2] == "FA" {
		return w.getFCTPrivateKey(address)
	} else if address[:2] == "EC" {
		return w.getECPrivateKey(address)
	}

	return "", fmt.Errorf("Not a public address")
}

func (w *WalletDB) getECPrivateKey(address string) (secret string, err error) {
	adds, err := w.Wallet.GetAllECAddresses()
	if err != nil {
		return "", err
	}

	for _, ec := range adds {
		if strings.Compare(ec.String(), address) == 0 {
			return ec.SecString(), nil
		}
	}

	return "", fmt.Errorf("Address not found")
}

func (w *WalletDB) getFCTPrivateKey(address string) (secret string, err error) {
	adds, err := w.Wallet.GetAllFCTAddresses()
	if err != nil {
		return "", err
	}

	for _, fa := range adds {
		if strings.Compare(fa.String(), address) == 0 {
			return fa.SecString(), nil
		}
	}

	return "", fmt.Errorf("Address not found")
}

func (w *WalletDB) GenerateEntryCreditAddress(name string) (*address.AddressNamePair, error) {
	address, err := w.Wallet.GenerateECAddress()
	if err != nil {
		return nil, err
	}

	anp, err := w.guiWallet.AddAddress(name, address.String(), 2)
	if err != nil {
		return nil, err
	}

	w.Save()
	if err != nil {
		return nil, err
	}

	return anp, nil
}

// TODO: Fix, make guiwallet take the remove
func (w *WalletDB) RemoveAddress(address string) (*address.AddressNamePair, error) {
	anp, err := w.guiWallet.RemoveAddress(address)
	if err != nil {
		return nil, err
	}

	err = w.Save()
	if err != nil {
		return nil, err
	}

	return anp, nil
}

func (w *WalletDB) AddAddress(name string, secret string) (*address.AddressNamePair, error) {
	if !factom.IsValidAddress(secret) {
		return nil, fmt.Errorf("Not a valid private key")
	} else if secret[:2] == "Fs" {
		add, err := factom.GetFactoidAddress(secret)
		if err != nil {
			return nil, err
		}

		err = w.Wallet.InsertFCTAddress(add)
		if err != nil {
			return nil, err
		}

		anp, err := w.addGUIAddress(name, add.String())
		if err != nil {
			return nil, err
		}

		err = w.Save()
		if err != nil {
			return nil, err
		}

		return anp, nil
	} else if secret[:2] == "Es" {
		add, err := factom.GetECAddress(secret)
		if err != nil {
			return nil, err
		}

		err = w.Wallet.InsertECAddress(add)
		if err != nil {
			return nil, err
		}

		anp, err := w.addGUIAddress(name, add.String())
		if err != nil {
			return nil, err
		}

		err = w.Save()
		if err != nil {
			return nil, err
		}

		return anp, nil
	}
	return nil, fmt.Errorf("Not a valid private key")
}

// Only adds to GUI Database
func (w *WalletDB) addBatchGUIAddresses(names []string, addresses []string) error {
	if len(names) != len(addresses) {
		return fmt.Errorf("List length does not match")
	}

	for i := 0; i < len(names); i++ {
		w.addGUIAddress(names[i], addresses[i])
	}

	return w.Save()
}

// Only adds to GUI database
func (w *WalletDB) addGUIAddress(name string, address string) (*address.AddressNamePair, error) {
	anp, err := w.guiWallet.AddAddress(name, address, 3)
	if err != nil {
		return nil, err
	}
	err = w.Save()
	if err != nil {
		return nil, err
	}

	return anp, nil
}

// Returns address with associated name
// List is 0 for not found, 1 for Factoid address, 2 for EC Address, 3 for External
func (w *WalletDB) GetGUIAddress(address string) (anp *address.AddressNamePair, list int) {
	anp, list, _ = w.guiWallet.GetAddress(address)
	return
}

func (w *WalletDB) ChangeAddressName(address string, toName string) error {
	err := w.guiWallet.ChangeAddressName(address, toName)
	if err != nil {
		return err
	}
	return w.Save()
}

func (w *WalletDB) GetTotalGUIAddresses() uint32 {
	return w.guiWallet.GetTotalAddressCount()
}

func (w *WalletDB) GetAllGUIAddresses() []address.AddressNamePair {
	return w.guiWallet.GetAllAddresses()
}

func GetHomeDir() string {
	// Get the OS specific home directory via the Go standard lib.
	var homeDir string
	usr, err := user.Current()
	if err == nil {
		homeDir = usr.HomeDir
	}

	// Fall back to standard HOME environment variable that works
	// for most POSIX OSes if the directory from the Go standard
	// lib failed.
	if err != nil || homeDir == "" {
		homeDir = os.Getenv("HOME")
	}
	return homeDir
}

// Wallet use outside DB
type WalletStruct struct {
	FactoidAddresses     *address.AddressList
	EntryCreditAddresses *address.AddressList
	ExternalAddresses    *address.AddressList

	sync.RWMutex
}

func NewWallet() *WalletStruct {
	w := new(WalletStruct)
	w.FactoidAddresses = address.NewAddressList()
	w.EntryCreditAddresses = address.NewAddressList()
	w.ExternalAddresses = address.NewAddressList()

	return w
}

func (w *WalletStruct) AddAddress(name string, address string, list int) (*address.AddressNamePair, error) {
	if list > 3 || list <= 0 {
		return nil, fmt.Errorf("Invalid list")
	}

	w.Lock()
	defer w.Unlock()

	switch list {
	case 1:
		return w.FactoidAddresses.Add(name, address)
	case 2:
		return w.EntryCreditAddresses.Add(name, address)
	case 3:
		return w.ExternalAddresses.Add(name, address)
	}

	return nil, fmt.Errorf("Encountered an error, this should not be able to happen")
}

func (w *WalletStruct) GetTotalAddressCount() uint32 {
	w.RLock()
	defer w.RUnlock()
	return w.FactoidAddresses.Length + w.EntryCreditAddresses.Length + w.ExternalAddresses.Length
}

// List is 0 for not found, 1 for FactoidAddressList, 2 for EntryCreditList, 3 for External
func (w *WalletStruct) GetAddress(address string) (anp *address.AddressNamePair, list int, index int) {
	w.RLock()
	defer w.RUnlock()

	list = 0

	anp, index = w.FactoidAddresses.Get(address)
	if index != -1 && anp != nil {
		list = 1
		return
	}

	anp, index = w.EntryCreditAddresses.Get(address)
	if index != -1 && anp != nil {
		list = 2
		return
	}

	anp, index = w.ExternalAddresses.Get(address)
	if index != -1 && anp != nil {
		list = 3
		return
	}

	return
}

func (w *WalletStruct) ChangeAddressName(address string, toName string) error {
	anp, list, i := w.GetAddress(address)

	w.Lock()
	defer w.Unlock()
	if strings.Compare(anp.Address, address) == 0 { // To be sure
		switch list {
		case 1:
			w.FactoidAddresses.List[i].Name = toName
			return nil
		case 2:
			w.EntryCreditAddresses.List[i].Name = toName
			return nil
		case 3:
			w.ExternalAddresses.List[i].Name = toName
			return nil
		}
	}

	return fmt.Errorf("Could not change name")
}

func (w *WalletStruct) GetAllAddresses() []address.AddressNamePair {
	w.RLock()
	defer w.RUnlock()
	var anpList []address.AddressNamePair
	anpList = append(anpList, w.FactoidAddresses.List...)
	anpList = append(anpList, w.EntryCreditAddresses.List...)
	anpList = append(anpList, w.ExternalAddresses.List...)

	return anpList
}

func (w *WalletStruct) IsSameAs(b *WalletStruct) bool {
	w.RLock()
	defer w.RUnlock()
	b.RLock()
	defer b.RUnlock()

	if !w.FactoidAddresses.IsSameAs(b.FactoidAddresses) {
		return false
	} else if !w.EntryCreditAddresses.IsSameAs(b.EntryCreditAddresses) {
		return false
	} else if !w.ExternalAddresses.IsSameAs(b.ExternalAddresses) {
		return false
	}
	return true
}

func (w *WalletStruct) MarshalBinary() ([]byte, error) {
	w.RLock()
	defer w.RUnlock()
	buf := new(bytes.Buffer)

	data, err := w.FactoidAddresses.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buf.Write(data)

	data, err = w.EntryCreditAddresses.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buf.Write(data)

	data, err = w.ExternalAddresses.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buf.Write(data)

	return buf.Next(buf.Len()), nil
}

func (w *WalletStruct) UnmarshalBinaryData(data []byte) (newData []byte, err error) {
	w.Lock()
	defer w.Unlock()
	newData = data
	newData, err = w.FactoidAddresses.UnmarshalBinaryData(newData)
	if err != nil {
		return
	}

	newData, err = w.EntryCreditAddresses.UnmarshalBinaryData(newData)
	if err != nil {
		return
	}

	newData, err = w.ExternalAddresses.UnmarshalBinaryData(newData)
	if err != nil {
		return
	}

	return
}

func (w *WalletStruct) UnmarshalBinary(data []byte) error {
	_, err := w.UnmarshalBinaryData(data)
	return err
}

func (w *WalletStruct) RemoveAddress(address string) (*address.AddressNamePair, error) {
	anp, list, _ := w.GetAddress(address)
	if list > 3 {
		return nil, fmt.Errorf("This should never happen")
	}

	w.Lock()
	defer w.Unlock()

	switch list {
	case 0:
		return nil, fmt.Errorf("No address found")
	case 1:
		err := w.FactoidAddresses.Remove(anp)
		if err != nil {
			return nil, err
		}

		// factom-wallet remove?
		return anp, nil
	case 2:
		err := w.EntryCreditAddresses.Remove(anp)
		if err != nil {
			return nil, err
		}

		// factom-wallet remove?
		return anp, nil
	case 3:
		err := w.ExternalAddresses.Remove(anp)
		if err != nil {
			return nil, err
		}

		// factom-wallet remove?
		return anp, nil
	}

	return nil, fmt.Errorf("Impossible to reach.")
}

// Adds balances to addresses so the GUI can display
func (w *WalletStruct) addBalancesToAddresses() {
	w.Lock()
	defer w.Unlock()

	if w.FactoidAddresses.Length > 0 {
		for i, fa := range w.FactoidAddresses.List {
			bal, err := factom.GetFactoidBalance(fa.Address)
			if err != nil {
				w.FactoidAddresses.List[i].Balance = -1
			} else {
				w.FactoidAddresses.List[i].Balance = float64(bal) / 1e8
			}
		}

		for i, ec := range w.EntryCreditAddresses.List {
			bal, err := factom.GetECBalance(ec.Address)
			if err != nil {
				w.EntryCreditAddresses.List[i].Balance = -1
			} else {
				w.EntryCreditAddresses.List[i].Balance = float64(bal)
			}
		}

		for i, a := range w.ExternalAddresses.List {
			if a.Address[:2] == "FA" {
				bal, err := factom.GetFactoidBalance(a.Address)
				if err != nil {
					w.ExternalAddresses.List[i].Balance = -1
				} else {
					w.ExternalAddresses.List[i].Balance = float64(bal) / 1e8
				}
			} else if a.Address[:2] == "EC" {
				bal, err := factom.GetECBalance(a.Address)
				if err != nil {
					w.ExternalAddresses.List[i].Balance = -1
				} else {
					w.ExternalAddresses.List[i].Balance = float64(bal)
				}
			}
		}
	}
}
