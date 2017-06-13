package main

import(
	"errors"
	"fmt"
	"math/big"
	"log"
	"net/rpc"
	"os"
	"io/ioutil"
	"encoding/json"
	"net/http"
	"net"
	"sync"
	"gopkg.in/urfave/cli.v1"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/ethdb"
	//"github.com/ethereum/go-ethereum/logger/glog"
)

var (

	app       *cli.App
	IPCPathFlag = cli.StringFlag{
		Name : "ipc",
		Usage : "make-up fund for sender",
		}

	StatePools map[string]*StatePool
	wg sync.WaitGroup
)

const DatabasePath = "daemon_db/"
const DaemonLogPath = "daemon_log/"

func NewApp(version, usage string) *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = ""
	//app.Authors = nil
	app.Email = ""
	app.Version = version
	app.Usage = usage
	return app
}

func init() {
	app = NewApp("0.2", "the evm command line interface")
	app.Flags = []cli.Flag{
		IPCPathFlag,
		}
	app.Action = run
}

func run(ctx *cli.Context) error {
	var endpoint string
	if ctx.GlobalString(IPCPathFlag.Name) != "" {
		endpoint = ctx.GlobalString(IPCPathFlag.Name)
		} else {
			fmt.Println("Please specified the named pipe path for IPC")
			return nil
			}
	arith := new(VmDaemon)
	rpc.Register(arith)
	rpc.HandleHTTP()
	os.Remove(endpoint)
	l, e := net.Listen("unix", endpoint)
	if e != nil {
		fmt.Println("listen error:", e)
		}
	http.Serve(l, nil)

	return nil
}

func main(){
	os.Mkdir(DaemonLogPath, 0777)
	StatePools = make(map[string]*StatePool)
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
		}
	
}

type VmDaemon int

func (self *StatePool) ExecTask(command TaskCommand) []byte{
	txHash := common.HexToHash(command.TxHash)
	logPath := ""
	if command.TxHash != "" {
		logPath = DaemonLogPath + txHash.String()
	}else {
		logPath = DaemonLogPath + "default"
	}
	f, e := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	
        if e != nil {
                fmt.Println("fatal")
        }
	log.SetOutput(f)

	if command.TxHash != "" {
		self.latestHash = txHash
	}

	var (
		logger = vm.NewStructLogger(nil)
		sender = common.HexToAddress(command.Sender)
		receiver = common.HexToAddress(command.Receiver)
	)
	self.statedb.StartRecord(txHash, common.HexToHash(command.BlockHash), 0)
	self.statedb.AddBalance(sender, StringToBig(command.Fund))
	runtimeConfig := runtime.Config{
		Origin:   sender,
		State:    self.statedb,
		GasLimit: 10000000000,
		GasPrice: big.NewInt(0),
		Value:    StringToBig(command.Value),
		Time: StringToBig(command.Time),
		Difficulty: StringToBig(command.Difficulty),
		Coinbase: common.HexToAddress(command.Coinbase),
		BlockNumber: StringToBig(command.BlockNumber),
		EVMConfig: vm.Config{
			Tracer:             logger,
			Debug:              false,
			DisableGasMetering: true,
		},
	}
	var ret []byte
	var  err error

	if command.Deploy{
		ret, _, err = runtime.Create(common.Hex2Bytes(command.Code), &runtimeConfig)
		
	} else{
		if command.Code != "" && self.statedb.GetCode(receiver) == nil{
			self.statedb.SetCode(receiver, common.Hex2Bytes(command.Code))
		}
		ret, err = runtime.Call(receiver, common.Hex2Bytes(command.Input), &runtimeConfig)
	}
	
	if err != nil{
		fmt.Println(err)
	}
	record := self.statedb.JournalRecord()
	for k, v := range record {
		log.Println("JournalRecord", k.Hex(), v, self.statedb.GetBalance(*k))
	}
	self.statedb.Commit(false)
	f.Close()
	self.mutex.Unlock()
	return ret
}

//WriteStates open the statefile from command.Path and write it to Statedb
func (t* VmDaemon) WriteStates(command WriteCommand, reply *string) error{
	states, ok := StatePools[command.Multisig]
	if !ok{
	*reply = "No stateFile"
		return errors.New("No stateFile")
	}
	f, err := ioutil.ReadFile(command.Path)
	if err != nil {
			return err
	}
	var dump state.Dump
	json.Unmarshal(f,&dump)
	states.mutex.Lock()
	ReadStateDB(states.statedb, dump)
	states.statedb.Commit(false)
	states.mutex.Unlock()
	return nil
}

func (t* VmDaemon) DeployContract(command TaskCommand, result *string) error{
	var states *StatePool
	states, ok := StatePools[command.Multisig]
	if !ok {
		db, err := ethdb.NewLDBDatabase(DatabasePath + command.Multisig, 1024, 0)
		if err != nil {
			fmt.Println(err)
		}
		statedb, _ := state.New(common.Hash{}, db)
		states = &StatePool{
			statedb: statedb,
			}
		StatePools[command.Multisig] = states
	}

	if command.SyncCall{
		states.mutex.Lock()
		ret := states.ExecTask(command)
		*result = fmt.Sprintf("%x", ret)
	} else{
		go func(){
			states.mutex.Lock()
			states.ExecTask(command)
		}()
	}
	return nil
}

func (t* VmDaemon) WriteLog(command LogCommand, result *string) error{
	//write logs to a [filename]
	states, ok := StatePools[command.Multisig]
	if ok{
		f, err := os.OpenFile(command.Path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
		states.statedb.Commit(false)
		f.WriteString(GetLogs(states.statedb))
	} else{
		return errors.New("not found")
	}
	return nil
}

func (t* VmDaemon) GetLatestTx(Multisig string, result *string) error{
	states, ok := StatePools[Multisig]
	if ok{
		*result = states.latestHash.Hex()
	} else{
		return errors.New("not found")
	}
	return nil	
}

func (t *VmDaemon) RemoveStates(Multisig string, result *string) error{
	states, ok := StatePools[Multisig]
	if ok {
		states.mutex.Lock()
		delete(StatePools, Multisig)
		os.RemoveAll(DatabasePath + Multisig)
		states.mutex.Unlock()
		*result = "remove " + Multisig
		return nil
	}
	*result = "The state does not exist"
	return nil
}

func (t *VmDaemon) IncNonce(command NonceCommand, result *string) error{
	states, ok := StatePools[command.Multisig]
	if ok {
		receiveradr := common.HexToAddress(command.Receiver)
		states.mutex.Lock()
		account := states.statedb.GetOrNewStateObject(receiveradr)
		account.SetNonce(account.Nonce() + 1)
		states.statedb.Commit(false)
		states.mutex.Unlock()
		*result = "increase nonce of " + command.Receiver
	}else {
		*result = "cannot find states on " + command.Multisig
	}
	return nil
}

func (t *VmDaemon) QueryStates(request QueryRequest, result *string) error{
	var states *StatePool
	states, ok := StatePools[request.Multisig]
	if !ok {
		return nil
		}
	states.mutex.Lock()
	*result = string(states.statedb.Dump())
	states.mutex.Unlock()
	return nil
}


func ReadStateDB(statedb *state.StateDB,world state.Dump) {
	for key, value := range world.Accounts{
		address := common.HexToAddress(key)
		statedb.SetCode(address, common.Hex2Bytes(value.Code))
		statedb.SetBalance(address, StringToBig(value.Balance))
		statedb.SetNonce(address, value.Nonce)
		for storage_location, storage_value := range value.Storage{
			statedb.SetState(address, common.HexToHash(storage_location), common.HexToHash(storage_value))
		}
	}
}

type NonceCommand struct{
	Multisig string
	Receiver string
}
type WriteCommand struct{
	Multisig string
	Path string
}

type QueryRequest struct{
	Multisig string
	Account string
}

type LogCommand struct{
	Multisig string
	Path string
}

type StatePool struct{
	statedb *state.StateDB
	latestHash common.Hash
	mutex sync.Mutex
	busy bool
}

type TaskCommand struct{
	Sender string
	Input string
	Receiver string
	Code string
	Value string
	Fund string
	Multisig string
	Time string
	TxHash string
	BlockHash string
	Coinbase string
	BlockNumber string
	Difficulty string
	Deploy bool
	SyncCall bool
}

func StringToBig(s string) *big.Int{
	b := big.NewInt(0)
	b.SetString(s, 10)
	return b
}


func GetLogs(statedb *state.StateDB) string {
	var logsStr string
    logsStr += `{ "logs":[`
	count := 0

	for _, lg := range statedb.Logs() {
        count += 1
        if count != 1 {
            logsStr += ", "
        }
		logsStr += lg.JsonString()
		}
    logsStr += "]}"

	return logsStr
}
