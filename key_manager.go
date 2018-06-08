package vault

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	crypto "github.com/thetatoken/theta/go-crypto"
	"github.com/thetatoken/theta/go-crypto/keys"
	"github.com/thetatoken/theta/types"
)

type Record struct {
	UserID     string
	Address    string
	PubKey     crypto.PubKey
	PrivateKey crypto.PrivKey
	Type       string
}

type KeyManager interface {
	Close()
	FindByUserId(userid string) (Record, error)
	// FindByAddress(address string) (Record, error)
	Create(r Record) error
}

func Sign(pubKey crypto.PubKey, privKey crypto.PrivKey, tx keys.Signable) ([]byte, error) {
	sig := privKey.Sign(tx.SignBytes())
	err := tx.Sign(pubKey, sig)
	if err != nil {
		return nil, err
	}
	return tx.TxBytes()
}

func genKey() (address string, pubkey crypto.PubKey, privKey crypto.PrivKey, seed string, err error) {
	privKey = crypto.GenPrivKeyEd25519().Wrap()
	pubkey = privKey.PubKey()
	address = hex.EncodeToString(pubkey.Address())
	codec := keys.MustLoadCodec("english")
	words, err := codec.BytesToWords(privKey.Bytes())
	seed = strings.Join(words, " ")
	return
}

// ----------------- SQL KeyManager ---------------------

var _ KeyManager = SqlKeyManager{}

const TableName = "user_theta_native_wallet"

type SqlKeyManager struct {
	db *sql.DB
}

func NewSqlKeyManager(db *sql.DB) (*SqlKeyManager, error) {
	return &SqlKeyManager{db}, nil
}

func (km SqlKeyManager) FindByUserId(userid string) (Record, error) {
	query := fmt.Sprintf("SELECT privkey::bytea, pubkey::bytea, address::bytea FROM %s WHERE userid=$1", TableName)
	row := km.db.QueryRow(query, userid)

	var privkeyBytes, pubkeyBytes, address []byte
	err := row.Scan(&privkeyBytes, &pubkeyBytes, &address)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("No record with user ID: %s. Creating keys.", userid)

		address, pubkey, privkey, _, err := genKey()
		if err != nil {
			return Record{}, err
		}
		record := Record{
			Address:    address,
			PubKey:     pubkey,
			PrivateKey: privkey,
			UserID:     userid,
		}
		err = km.Create(record)
		if err != nil {
			log.WithError(err).WithField("userid", userid).Error("Failed to create address")
			return Record{}, err
		}
		return record, nil
	case err != nil:
		log.Printf(err.Error())
		return Record{}, err
	default:
		pubKey := crypto.PubKey{}
		types.FromBytes(pubkeyBytes, &pubKey)
		privKey := crypto.PrivKey{}
		types.FromBytes(privkeyBytes, &privKey)

		record := Record{
			UserID:     userid,
			PubKey:     pubKey,
			PrivateKey: privKey,
			Address:    hex.EncodeToString(address),
		}
		return record, nil
	}
}

func (km SqlKeyManager) Close() {}

func (km SqlKeyManager) Create(record Record) error {
	sm := fmt.Sprintf("INSERT INTO %s (userid, pubkey, privkey, address) VALUES ($1, DECODE($2, 'hex'), DECODE($3, 'hex'), DECODE($4, 'hex'))", TableName)

	pubkeyBytes, err := types.ToBytes(&record.PubKey)
	if err != nil {
		return err
	}
	privBytes, err := types.ToBytes(&record.PrivateKey)
	if err != nil {
		return err
	}

	_, err = km.db.Exec(sm, record.UserID, hex.EncodeToString(pubkeyBytes), hex.EncodeToString(privBytes), record.Address)
	return err
}
