package e2etest

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/anon-org/anon/v4/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"

	"github.com/stretchr/testify/require"
)

type anonNode struct {
	cmd          *exec.Cmd
	pidFile      string
	dataDir      string
	nodeHome     string
	chainID      string
	slashingAddr string
	covenantPk   *types.BIP340PubKey
}

func newAnonNode(dataDir, nodeHome string, cmd *exec.Cmd, chainID string, slashingAddr string, covenantPk *types.BIP340PubKey) *anonNode {
	return &anonNode{
		dataDir:      dataDir,
		nodeHome:     nodeHome,
		cmd:          cmd,
		chainID:      chainID,
		slashingAddr: slashingAddr,
		covenantPk:   covenantPk,
	}
}

func (n *anonNode) start() error {
	if err := n.cmd.Start(); err != nil {
		return err
	}

	pid, err := os.Create(filepath.Join(n.dataDir,
		fmt.Sprintf("%s.pid", "config")))
	if err != nil {
		return err
	}

	n.pidFile = pid.Name()
	if _, err = fmt.Fprintf(pid, "%d\n", n.cmd.Process.Pid); err != nil {
		return err
	}

	if err := pid.Close(); err != nil {
		return err
	}

	return nil
}

func (n *anonNode) stop() (err error) {
	if n.cmd == nil || n.cmd.Process == nil {
		// return if not properly initialized
		// or error starting the process
		return nil
	}

	defer func() {
		err = n.cmd.Wait()
	}()

	if runtime.GOOS == "windows" {
		return n.cmd.Process.Signal(os.Kill)
	}

	return n.cmd.Process.Signal(os.Interrupt)
}

func (n *anonNode) cleanup() error {
	if n.pidFile != "" {
		if err := os.Remove(n.pidFile); err != nil {
			log.Printf("unable to remove file %s: %v", n.pidFile,
				err)
		}
	}

	dirs := []string{
		n.dataDir,
	}
	var err error
	for _, dir := range dirs {
		if err = os.RemoveAll(dir); err != nil {
			log.Printf("Cannot remove dir %s: %v", dir, err)
		}
	}

	return nil
}

func (n *anonNode) shutdown() error {
	if err := n.stop(); err != nil {
		return err
	}
	if err := n.cleanup(); err != nil {
		return err
	}

	return nil
}

type AnonNodeHandler struct {
	anonNode *anonNode
}

func NewAnonNodeHandler(t *testing.T, covenantPk *types.BIP340PubKey) *AnonNodeHandler {
	testDir, err := baseDir("zAnonTest")
	require.NoError(t, err)
	defer func() {
		if err != nil {
			err := os.RemoveAll(testDir)
			require.NoError(t, err)
		}
	}()

	nodeHome := filepath.Join(testDir, "node0", "anond")

	slashingAddr := "SZtRT4BySL3o4efdGLh3k7Kny8GAnsBrSW"
	decodedAddr, err := btcutil.DecodeAddress(slashingAddr, &chaincfg.SimNetParams)
	require.NoError(t, err)
	pkScript, err := txscript.PayToAddrScript(decodedAddr)
	require.NoError(t, err)
	//nolint:noctx
	initTestnetCmd := exec.Command(
		"anond",
		"testnet",
		"--v=1",
		fmt.Sprintf("--output-dir=%s", testDir),
		"--starting-ip-address=192.168.10.2",
		"--keyring-backend=test",
		"--chain-id=chain-test",
		"--additional-sender-account",
		"--min-staking-time-blocks=100",
		"--min-staking-amount-sat=10000",
		// default checkpoint finalization timeout is 20, so we set min unbonding time
		// to be 1 block more
		"--unbonding-time=21",
		fmt.Sprintf("--slashing-pk-script=%s", hex.EncodeToString(pkScript)),
		fmt.Sprintf("--covenant-pks=%s", covenantPk.MarshalHex()),
		"--covenant-quorum=1",
	)

	var stderr bytes.Buffer
	initTestnetCmd.Stderr = &stderr

	err = initTestnetCmd.Run()
	if err != nil {
		fmt.Printf("init testnet failed: %s \n", stderr.String())
	}
	require.NoError(t, err)

	f, err := os.Create(filepath.Join(testDir, "anon.log"))
	require.NoError(t, err)
	//nolint:noctx
	startCmd := exec.Command("anond", "start",
		fmt.Sprintf("--home=%s", nodeHome),
		"--log_level=debug")
	startCmd.Env = append(os.Environ(), "ANON_BLS_PASSWORD=password")

	startCmd.Stdout = f

	return &AnonNodeHandler{
		anonNode: newAnonNode(testDir, nodeHome, startCmd, chainID, slashingAddr, covenantPk),
	}
}

func (w *AnonNodeHandler) Start() error {
	if err := w.anonNode.start(); err != nil {
		// try to cleanup after start error, but return original error
		_ = w.anonNode.cleanup()

		return err
	}

	return nil
}

func (w *AnonNodeHandler) Stop() error {
	if err := w.anonNode.shutdown(); err != nil {
		return err
	}

	return nil
}

func (w *AnonNodeHandler) GetNodeDataDir() string {
	dir := filepath.Join(w.anonNode.dataDir, "node0", "anond")

	return dir
}

func (w *AnonNodeHandler) GetCovenantPk() *types.BIP340PubKey {
	return w.anonNode.covenantPk
}
