package daemon

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/cometbft/cometbft/crypto/tmhash"
	sdkflags "github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/spf13/cobra"

	ancparams "github.com/anon-org/anon/v4/app/params"
	anctypes "github.com/anon-org/anon/v4/types"

	"github.com/anon-org/finality-provider/codec"
	"github.com/anon-org/finality-provider/eotsmanager"
	"github.com/anon-org/finality-provider/eotsmanager/config"
	"github.com/anon-org/finality-provider/log"
)

const (
	flagHomeNtk           = "ntk-home"
	flagKeyNameNtk        = "ntk-key-name"
	flagKeyringBackendNtk = "ntk-keyring-backend"
	flagMessage            = "message"
	flagOutputFile         = "output-file"
)

func init() {
	ancparams.SetAddressPrefixes()
}

// PoPExport the data needed to prove ownership of the eots and ntk key pairs.
type PoPExport struct {
	// Btc public key is the EOTS PK *anctypes.BIP340PubKey marshal hex
	EotsPublicKey string `json:"eotsPublicKey"`
	// Ntk public key is the *secp256k1.PubKey marshal hex
	NtkPublicKey string `json:"ntkPublicKey"`

	// Anon key pair signs EOTS public key as hex
	NtkSignEotsPk string `json:"ntkSignEotsPk"`
	// Schnorr signature of EOTS private key over the SHA256(Ntk address)
	EotsSignNtk string `json:"eotsSignNtk"`

	// Anon address ex.: anc1f04czxeqprn0s9fe7kdzqyde2e6nqj63dllwsm
	NtkAddress string `json:"ntkAddress"`
}

// PoPExportDelete the data needed to delete an ownership previously created.
type PoPExportDelete struct {
	// Btc public key is the EOTS PK *anctypes.BIP340PubKey marshal hex
	EotsPublicKey string `json:"eotsPublicKey"`
	// Ntk public key is the *secp256k1.PubKey marshal hex
	NtkPublicKey string `json:"ntkPublicKey"`

	// Anon key pair signs message
	NtkSignature string `json:"ntkSignature"`

	// Anon address ex.: anc1f04czxeqprn0s9fe7kdzqyde2e6nqj63dllwsm
	NtkAddress string `json:"ntkAddress"`
}

func NewPopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pop",
		Short: "Proof of Possession commands",
	}

	cmd.AddCommand(
		NewPopExportCmd(),
		NewPopDeleteCmd(),
		NewPopValidateExportCmd(),
	)

	return cmd
}

func NewPopExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Exports the Proof of Possession by (1) signing over the NTK address with the EOTS private key and (2) signing over the EOTS public key with the NTK private key.",
		Long: `Parse the address from the NTK keyring, load the address, hash it with
		sha256 and sign based on the EOTS key associated with the key-name or eots-pk flag.
		If the both flags are supplied, eots-pk takes priority. Use the generated signature
		to build a Proof of Possession. For the creation of the NTK signature over the eots pk,
		it loads the NTK key pair and signs the eots-pk hex and exports it.`,
		RunE: exportPop,
	}

	f := cmd.Flags()

	f.String(sdkflags.FlagHome, config.DefaultEOTSDir, "EOTS home directory")
	f.String(keyNameFlag, "", "EOTS key name")
	f.String(eotsPkFlag, "", "EOTS public key of the finality-provider")
	f.String(sdkflags.FlagKeyringBackend, keyring.BackendTest, "EOTS backend of the keyring")

	f.String(flagHomeNtk, "", "NTK home directory")
	f.String(flagKeyNameNtk, "", "NTK key name")
	f.String(flagKeyringBackendNtk, keyring.BackendTest, "NTK backend of the keyring")

	f.String(flagOutputFile, "", "Path to output JSON file")

	return cmd
}

func NewPopValidateExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "Validates the PoP of the pop export command.",
		Long:    `Receives as an argument the file path of the JSON output of the command eotsd pop export`,
		Example: `eotsd pop validate /path/to/pop.json`,
		RunE:    validatePop,
		Args:    cobra.ExactArgs(1),
	}

	return cmd
}

func NewPopDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Generate the delete data for removing a proof of possession previously created.",
		Long: `Parse the message from the flag --message and sign with the NTK keyring, it also loads
		the EOTS public key based on the EOTS key associated with the key-name or eots-pk flag.
		If the both flags are supplied, eots-pk takes priority.`,
		RunE: deletePop,
	}

	f := cmd.Flags()

	f.String(sdkflags.FlagHome, config.DefaultEOTSDir, "EOTS home directory")
	f.String(keyNameFlag, "", "EOTS key name")
	f.String(eotsPkFlag, "", "EOTS public key of the finality-provider")
	f.String(sdkflags.FlagKeyringBackend, keyring.BackendTest, "EOTS backend of the keyring")

	f.String(flagHomeNtk, "", "NTK home directory")
	f.String(flagKeyNameNtk, "", "NTK key name")
	f.String(flagKeyringBackendNtk, keyring.BackendTest, "NTK backend of the keyring")

	f.String(flagMessage, "", "Message to be signed")

	f.String(flagOutputFile, "", "Path to output JSON file")

	return cmd
}

func validatePop(cmd *cobra.Command, args []string) error {
	path := args[0]

	// Add path validation
	// #nosec G304 - The file path is provided by the user and not externally
	bzExportJSON, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to read pop file: %w", err)
	}

	var pop PoPExport
	if err := json.Unmarshal(bzExportJSON, &pop); err != nil {
		return fmt.Errorf("failed to marshal %s into PoPExport structure", string(bzExportJSON))
	}

	valid, err := ValidPopExport(pop)
	if err != nil {
		return fmt.Errorf("failed to validate pop %+v, reason: %w", pop, err)
	}
	if !valid {
		return fmt.Errorf("invalid pop %+v", pop)
	}

	cmd.Println("Proof of Possession is valid!")

	return nil
}

func exportPop(cmd *cobra.Command, _ []string) error {
	eotsHomePath, eotsKeyName, eotsFpPubKeyStr, eotsKeyringBackend, err := eotsFlags(cmd)
	if err != nil {
		return err
	}

	ntkHomePath, ntkKeyName, ntkKeyringBackend, err := ntkFlags(cmd)
	if err != nil {
		return err
	}

	ntkKeyring, ntkPubKey, ancAddr, err := ntkKeyring(ntkHomePath, ntkKeyName, ntkKeyringBackend, cmd.InOrStdin())
	if err != nil {
		return err
	}

	eotsManager, err := loadEotsManager(eotsHomePath, eotsFpPubKeyStr, eotsKeyName, eotsKeyringBackend)
	if err != nil {
		return err
	}
	defer cmdCloseEots(cmd, eotsManager)

	ancAddrStr := ancAddr.String()
	hashOfMsgToSign := tmhash.Sum([]byte(ancAddrStr))
	schnorrSigOverNtkAddr, eotsPk, err := eotsSignMsg(eotsManager, eotsKeyName, eotsFpPubKeyStr, hashOfMsgToSign)
	if err != nil {
		return fmt.Errorf("failed to sign address %s: %w", ancAddrStr, err)
	}

	eotsPkHex := eotsPk.MarshalHex()
	ntkSignature, err := SignCosmosAdr36(ntkKeyring, ntkKeyName, ancAddrStr, []byte(eotsPkHex))
	if err != nil {
		return err
	}

	out := PoPExport{
		EotsPublicKey: eotsPkHex,
		NtkPublicKey: base64.StdEncoding.EncodeToString(ntkPubKey.Bytes()),

		NtkAddress: ancAddrStr,

		EotsSignNtk:   base64.StdEncoding.EncodeToString(schnorrSigOverNtkAddr.Serialize()),
		NtkSignEotsPk: base64.StdEncoding.EncodeToString(ntkSignature),
	}

	return handleOutputJSON(cmd, out)
}

func deletePop(cmd *cobra.Command, _ []string) error {
	eotsHomePath, eotsKeyName, eotsFpPubKeyStr, eotsKeyringBackend, err := eotsFlags(cmd)
	if err != nil {
		return err
	}

	ntkHomePath, ntkKeyName, ntkKeyringBackend, err := ntkFlags(cmd)
	if err != nil {
		return err
	}

	ntkKeyring, ntkPubKey, ancAddr, err := ntkKeyring(ntkHomePath, ntkKeyName, ntkKeyringBackend, cmd.InOrStdin())
	if err != nil {
		return err
	}

	eotsManager, err := loadEotsManager(eotsHomePath, eotsFpPubKeyStr, eotsKeyName, eotsKeyringBackend)
	if err != nil {
		return err
	}
	defer cmdCloseEots(cmd, eotsManager)

	btcPubKey, err := eotsPubKey(eotsManager, eotsKeyName, eotsFpPubKeyStr)
	if err != nil {
		return fmt.Errorf("failed to get eots pk %w", err)
	}

	interpretedMsg, err := getInterpretedMessage(cmd)
	if err != nil {
		return err
	}

	ancAddrStr := ancAddr.String()
	ntkSignature, err := SignCosmosAdr36(ntkKeyring, ntkKeyName, ancAddrStr, []byte(interpretedMsg))
	if err != nil {
		return err
	}

	out := PoPExportDelete{
		EotsPublicKey: btcPubKey.MarshalHex(),
		NtkPublicKey: base64.StdEncoding.EncodeToString(ntkPubKey.Bytes()),

		NtkAddress: ancAddrStr,

		NtkSignature: base64.StdEncoding.EncodeToString(ntkSignature),
	}

	return handleOutputJSON(cmd, out)
}

// ntkFlags returns the values of flagHomeNtk, flagKeyNameNtk and
// flagKeyringBackendNtk respectively or error if something fails
func ntkFlags(cmd *cobra.Command) (string, string, string, error) {
	f := cmd.Flags()

	ntkHomePath, err := getCleanPath(cmd, flagHomeNtk)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to load ntk home flag: %w", err)
	}

	ntkKeyName, err := f.GetString(flagKeyNameNtk)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get ntk key name: %w", err)
	}

	ntkKeyringBackend, err := f.GetString(flagKeyringBackendNtk)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get ntk keyring backend: %w", err)
	}

	return ntkHomePath, ntkKeyName, ntkKeyringBackend, nil
}

// eotsFlags returns the values of FlagHome, keyNameFlag,
// eotsPkFlag, FlagKeyringBackend respectively or error
// if something fails
func eotsFlags(cmd *cobra.Command) (string, string, string, string, error) {
	f := cmd.Flags()

	eotsKeyName, err := f.GetString(keyNameFlag)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get eots key name: %w", err)
	}

	eotsFpPubKeyStr, err := f.GetString(eotsPkFlag)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get eots public key: %w", err)
	}

	eotsKeyringBackend, err := f.GetString(sdkflags.FlagKeyringBackend)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get eots keyring backend: %w", err)
	}

	eotsHomePath, err := getHomePath(cmd)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to get eots home path: %w", err)
	}

	return eotsHomePath, eotsKeyName, eotsFpPubKeyStr, eotsKeyringBackend, nil
}

func getInterpretedMessage(cmd *cobra.Command) (string, error) {
	msg, err := cmd.Flags().GetString(flagMessage)
	if err != nil {
		return "", fmt.Errorf("failed to get message flag: %w", err)
	}
	if len(msg) == 0 {
		return "", fmt.Errorf("flage --%s is empty", flagMessage)
	}

	// We are assuming we are receiving string literal with escape characters
	interpretedMsg, err := strconv.Unquote(`"` + msg + `"`)
	if err != nil {
		return "", fmt.Errorf("failed to unquote message: %w", err)
	}

	return interpretedMsg, nil
}

func handleOutputJSON(cmd *cobra.Command, out any) error {
	jsonBz, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	outputFilePath, err := cmd.Flags().GetString(flagOutputFile)
	if err != nil {
		return fmt.Errorf("failed to get output file path: %w", err)
	}

	if len(outputFilePath) > 0 {
		// Add path validation
		cleanPath, err := filepath.Abs(filepath.Clean(outputFilePath))
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(cleanPath)
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		if err := os.WriteFile(cleanPath, jsonBz, 0600); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
	}

	cmd.Println(string(jsonBz))

	return nil
}

func ntkKeyring(
	ntkHomePath, ntkKeyName, ntkKeyringBackend string,
	userInput io.Reader,
) (keyring.Keyring, *secp256k1.PubKey, sdk.AccAddress, error) {
	cdc := codec.MakeCodec()
	ntkKeyring, err := keyring.New("ntk", ntkKeyringBackend, ntkHomePath, userInput, cdc)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create keyring: %w", err)
	}

	ntkKeyRecord, err := ntkKeyring.Key(ntkKeyName)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get ntk keyring: %w", err)
	}

	ntkPubKey, err := ntkPk(ntkKeyRecord)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get ntk public key: %w", err)
	}

	return ntkKeyring, ntkPubKey, sdk.AccAddress(ntkPubKey.Address().Bytes()), nil
}

func loadEotsManager(eotsHomePath, eotsFpPubKeyStr, eotsKeyName, eotsKeyringBackend string) (*eotsmanager.LocalEOTSManager, error) {
	if len(eotsFpPubKeyStr) == 0 && len(eotsKeyName) == 0 {
		return nil, fmt.Errorf("at least one of the flags: %s, %s needs to be informed", keyNameFlag, eotsPkFlag)
	}

	cfg, err := config.LoadConfig(eotsHomePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config at %s: %w", eotsHomePath, err)
	}

	logger, err := log.NewRootLoggerWithFile(config.LogFile(eotsHomePath), cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to load the logger: %w", err)
	}

	dbBackend, err := cfg.DatabaseConfig.GetDBBackend()
	if err != nil {
		return nil, fmt.Errorf("failed to get db backend: %w", err)
	}

	eotsManager, err := eotsmanager.NewLocalEOTSManager(eotsHomePath, eotsKeyringBackend, dbBackend, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create EOTS manager: %w", err)
	}

	return eotsManager, nil
}

func SignCosmosAdr36(
	kr keyring.Keyring,
	keyName string,
	cosmosBech32Address string,
	bytesToSign []byte,
) ([]byte, error) {
	base64Bytes := base64.StdEncoding.EncodeToString(bytesToSign)

	signDoc := NewCosmosSignDoc(
		cosmosBech32Address,
		base64Bytes,
	)

	marshaled, err := json.Marshal(signDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sign doc: %w", err)
	}

	bz := sdk.MustSortJSON(marshaled)

	ntkSignBytes, _, err := kr.Sign(
		keyName,
		bz,
		signing.SignMode_SIGN_MODE_DIRECT,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to sign btc address bytes: %w", err)
	}

	return ntkSignBytes, nil
}

func ValidPopExport(pop PoPExport) (bool, error) {
	valid, err := ValidEotsSignNtk(pop.EotsPublicKey, pop.NtkAddress, pop.EotsSignNtk)
	if err != nil || !valid {
		return false, err
	}

	return ValidNtkSignEots(
		pop.NtkPublicKey,
		pop.NtkAddress,
		pop.EotsPublicKey,
		pop.NtkSignEotsPk,
	)
}

func ValidEotsSignNtk(eotsPk, ntkAddr, eotsSigOverNtkAddr string) (bool, error) {
	eotsPubKey, err := anctypes.NewBIP340PubKeyFromHex(eotsPk)
	if err != nil {
		return false, fmt.Errorf("failed to parse eots public key: %w", err)
	}

	schnorrSigBase64, err := base64.StdEncoding.DecodeString(eotsSigOverNtkAddr)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	schnorrSig, err := schnorr.ParseSignature(schnorrSigBase64)
	if err != nil {
		return false, fmt.Errorf("failed to parse schnorr signature: %w", err)
	}
	sha256Addr := tmhash.Sum([]byte(ntkAddr))

	return schnorrSig.Verify(sha256Addr, eotsPubKey.MustToBTCPK()), nil
}

func ValidNtkSignEots(ntkPk, ntkAddr, eotsPkHex, ntkSigOverEotsPk string) (bool, error) {
	ntkPubKeyBz, err := base64.StdEncoding.DecodeString(ntkPk)
	if err != nil {
		return false, fmt.Errorf("failed to parse ntk public key: %w", err)
	}

	ntkPubKey := &secp256k1.PubKey{
		Key: ntkPubKeyBz,
	}

	eotsPk, err := anctypes.NewBIP340PubKeyFromHex(eotsPkHex)
	if err != nil {
		return false, fmt.Errorf("failed to parse eots public key: %w", err)
	}

	ntkSignEots := []byte(eotsPk.MarshalHex())
	base64Bytes := base64.StdEncoding.EncodeToString(ntkSignEots)
	ntkSignBtcDoc := NewCosmosSignDoc(ntkAddr, base64Bytes)
	ntkSignBtcMarshaled, err := json.Marshal(ntkSignBtcDoc)
	if err != nil {
		return false, fmt.Errorf("failed to marshal sign doc: %w", err)
	}

	ntkSignEotsBz := sdk.MustSortJSON(ntkSignBtcMarshaled)

	secp256SigBase64, err := base64.StdEncoding.DecodeString(ntkSigOverEotsPk)
	if err != nil {
		return false, fmt.Errorf("failed to parse ntk signature: %w", err)
	}

	return ntkPubKey.VerifySignature(ntkSignEotsBz, secp256SigBase64), nil
}

func ntkPk(ntkRecord *keyring.Record) (*secp256k1.PubKey, error) {
	pubKey, err := ntkRecord.GetPubKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get ntk public key: %w", err)
	}

	switch v := pubKey.(type) {
	case *secp256k1.PubKey:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported key type in keyring: %T", pubKey)
	}
}

func eotsPubKey(
	eotsManager *eotsmanager.LocalEOTSManager,
	keyName, fpPkStr string,
) (*anctypes.BIP340PubKey, error) {
	if len(fpPkStr) > 0 {
		fpPk, err := anctypes.NewBIP340PubKeyFromHex(fpPkStr)
		if err != nil {
			return nil, fmt.Errorf("invalid finality-provider public key %s: %w", fpPkStr, err)
		}

		return fpPk, nil
	}

	fpPk, err := eotsManager.LoadBIP340PubKeyFromKeyName(keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to load EOTS public key from key name: %w", err)
	}

	return fpPk, nil
}

func eotsSignMsg(
	eotsManager *eotsmanager.LocalEOTSManager,
	keyName, fpPkStr string,
	hashOfMsgToSign []byte,
) (*schnorr.Signature, *anctypes.BIP340PubKey, error) {
	if len(fpPkStr) > 0 {
		fpPk, err := anctypes.NewBIP340PubKeyFromHex(fpPkStr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid finality-provider public key %s: %w", fpPkStr, err)
		}
		signature, err := eotsManager.SignSchnorrSig(*fpPk, hashOfMsgToSign)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to sign msg with pk %s: %w", fpPkStr, err)
		}

		return signature, fpPk, nil
	}

	signature, fpPk, err := eotsManager.SignSchnorrSigFromKeyname(keyName, hashOfMsgToSign)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to sign msg with key name %s: %w", keyName, err)
	}

	return signature, fpPk, nil
}

func cmdCloseEots(
	cmd *cobra.Command,
	eotsManager *eotsmanager.LocalEOTSManager,
) {
	err := eotsManager.Close()
	if err != nil {
		cmd.Printf("error closing eots manager: %s", err.Error())
	}
}

type Msg struct {
	Type  string   `json:"type"`
	Value MsgValue `json:"value"`
}

type SignDoc struct {
	ChainID       string `json:"chain_id"`
	AccountNumber string `json:"account_number"`
	Sequence      string `json:"sequence"`
	Fee           Fee    `json:"fee"`
	Msgs          []Msg  `json:"msgs"`
	Memo          string `json:"memo"`
}

type Fee struct {
	Gas    string   `json:"gas"`
	Amount []string `json:"amount"`
}

type MsgValue struct {
	Signer string `json:"signer"`
	Data   string `json:"data"`
}

func NewCosmosSignDoc(
	signer string,
	data string,
) *SignDoc {
	return &SignDoc{
		ChainID:       "",
		AccountNumber: "0",
		Sequence:      "0",
		Fee: Fee{
			Gas:    "0",
			Amount: []string{},
		},
		Msgs: []Msg{
			{
				Type: "sign/MsgSignData",
				Value: MsgValue{
					Signer: signer,
					Data:   data,
				},
			},
		},
		Memo: "",
	}
}
