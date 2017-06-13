package main

import(
	"net/rpc"
	"fmt"
	//"net/http"
	"log"
	"gopkg.in/urfave/cli.v1"
	"path/filepath"
	"os"
)
type Daem int

var (
	app       *cli.App
	IPCPathFlag = cli.StringFlag{
		Name : "ipc",
		Usage : "FIFO path",
		}
	SenderFlag = cli.StringFlag{
		Name : "sender",
		Usage : "sender address",
		}
	ReceiverFlag = cli.StringFlag{
		Name : "receiver",
		Usage : "receiver address",
		}
	CodeFlag = cli.StringFlag{
		Name : "code",
		Usage : "code to be deployed",
		}
	ValueFlag = cli.StringFlag{
		Name : "value",
		Usage : "value to be sent",
		}
	FundFlag = cli.StringFlag{
		Name : "fund",
		Usage : "make-up fund for sender",
		}
	MultisigAddressFlag = cli.StringFlag{
		Name : "multisig",
		Usage : "multisig",
		Value : "0",
		}
	TimeFlag = cli.StringFlag{
		Name : "time",
		Usage : "Time on the block",
		Value : "0",
	}
	InputFlag = cli.StringFlag{
		Name : "input",
		Usage : "input code",
		}
	DeployFlag = cli.BoolFlag{
		Name : "deploy",
		Usage : "deploy contract",
		}
	DumpFlag = cli.BoolFlag{
		Name : "dump",
		Usage : "dump states from the specified multisig's state",
	}
	WriteStateFlag = cli.StringFlag{
		Name : "writestate",
		Usage : "write the state to the account in the multisig's state",
	}
	RemoveFlag = cli.BoolFlag{
		Name : "remove",
		Usage : "Remove the multisig's state",
	}
	IncNonceFlag = cli.BoolFlag{
		Name : "inc",
		Usage : "Inc the receiver's nonce",
	}
	ReturnFlag = cli.BoolFlag{
		Name : "return",
		Usage : "will be an synchronous call",
	}
	LatestFlag = cli.BoolFlag{
		Name : "latest",
		Usage : "get the latest TxHash that daemon got",
	}
	WriteLogFlag = cli.StringFlag{
		Name:  "writelog",
		Usage: "wrtie logs to a file",
	}
	TxHashFlag = cli.StringFlag{
		Name: "txhash",
		Usage: "txhash will be written in the daemon log",
	}
	BlockHashFlag = cli.StringFlag{
		Name: "blockhash",
		Usage: "blockhash",
	}
	BlockNumberFlag = cli.StringFlag{
		Name: "blocknumber",
		Usage: "blocknumber",
	}
	DifficultyFlag = cli.StringFlag{
		Name: "difficulty",
		Usage: "difficulty",
	}
	CoinbaseFlag = cli.StringFlag{
		Name: "coinbase",
		Usage: "coinbase",
	}
	
)
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
		BlockHashFlag,
		BlockNumberFlag,
		CoinbaseFlag,
		CodeFlag,
		DifficultyFlag,
		DeployFlag,
		DumpFlag,
		FundFlag,
		IPCPathFlag,
		IncNonceFlag,
		LatestFlag,
		MultisigAddressFlag,
		ReceiverFlag,
		RemoveFlag,
		ReturnFlag,
		SenderFlag,
		TimeFlag,
		ValueFlag,
		InputFlag,
		WriteStateFlag,
		WriteLogFlag,
		TxHashFlag,
		}
	app.Action = run
}

func run(ctx *cli.Context) error {
	var endpoint string
	var reply string
	if ctx.GlobalString(IPCPathFlag.Name) != "" {
		endpoint = ctx.GlobalString(IPCPathFlag.Name)
		} else {
			fmt.Println("ipc path is required")
			return nil
		}

	client, err := rpc.DialHTTP("unix", endpoint)
	if ctx.GlobalBool(LatestFlag.Name) {
		err = client.Call("VmDaemon.GetLatestTx", ctx.GlobalString(MultisigAddressFlag.Name), &reply)
		fmt.Println(reply)
		return nil
	}
	if ctx.GlobalBool(RemoveFlag.Name) {
		err = client.Call("VmDaemon.RemoveStates", ctx.GlobalString(MultisigAddressFlag.Name), &reply)
		fmt.Println(reply)
		return nil
	}
	if ctx.GlobalString(WriteLogFlag.Name) != "" {
		command := LogCommand{
			Multisig : ctx.GlobalString(MultisigAddressFlag.Name),
			Path : ctx.GlobalString(WriteLogFlag.Name),
		}
		err = client.Call("VmDaemon.WriteLog", command, &reply)
		return nil
	}
	if ctx.GlobalBool(IncNonceFlag.Name) {
		command := NonceCommand{
			Multisig : ctx.GlobalString(MultisigAddressFlag.Name),
			Receiver : ctx.GlobalString(ReceiverFlag.Name),
		}
		err = client.Call("VmDaemon.IncNonce", command, &reply)
		fmt.Println(reply)
		return nil
	}
	if ctx.GlobalBool(DumpFlag.Name) {
		query := QueryRequest{
			Multisig : ctx.GlobalString(MultisigAddressFlag.Name),
			Account : ctx.GlobalString(ReceiverFlag.Name),
		}
		err = client.Call("VmDaemon.QueryStates", query, &reply)
		fmt.Println(reply)
		return nil
	}
	if ctx.GlobalString(WriteStateFlag.Name) != "" {
		writerequest := WriteCommand{
			Path : ctx.GlobalString(WriteStateFlag.Name),
			Multisig : ctx.GlobalString(MultisigAddressFlag.Name),
		}
		err = client.Call("VmDaemon.WriteStates", writerequest, &reply)
		fmt.Println(reply)
		return nil
	}
	task := TaskCommand{
		Sender : ctx.GlobalString(SenderFlag.Name),
		Receiver : ctx.GlobalString(ReceiverFlag.Name),
		Code : ctx.GlobalString(CodeFlag.Name),
		Value : ctx.GlobalString(ValueFlag.Name),
		Fund : ctx.GlobalString(FundFlag.Name),
		Multisig : ctx.GlobalString(MultisigAddressFlag.Name),
		Input : ctx.GlobalString(InputFlag.Name),
		Time : ctx.GlobalString(TimeFlag.Name),
		Deploy : ctx.GlobalBool(DeployFlag.Name),
		TxHash : ctx.GlobalString(TxHashFlag.Name),
		BlockHash : ctx.GlobalString(BlockHashFlag.Name),
		BlockNumber :ctx.GlobalString(BlockNumberFlag.Name),
		Coinbase : ctx.GlobalString(CoinbaseFlag.Name),
		Difficulty : ctx.GlobalString(DifficultyFlag.Name),
		SyncCall : ctx.GlobalBool(ReturnFlag.Name),
	}
	if err != nil {
		log.Fatal("dialing:", err)
		}
	err = client.Call("VmDaemon.DeployContract", task, &reply)
	fmt.Println(reply)

	if err != nil {
		log.Fatal("error:", err)
	}
	return nil
}

func main(){

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
		}

}

type WriteCommand struct{
	Multisig string
	Path string
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

type NonceCommand struct{
	Multisig string
	Receiver string
}

type QueryRequest struct{
	Multisig string
	Account string
}

type LogCommand struct{
	Multisig string
	Path string
}
