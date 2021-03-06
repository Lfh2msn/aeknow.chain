package main

import (
	//"encoding/json"
	"bufio"
	crypto_rand "crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	//"time"

	"github.com/aeternity/aepp-sdk-go/v7/account"
	aeconfig "github.com/aeternity/aepp-sdk-go/v7/config"
	"github.com/aeternity/aepp-sdk-go/v7/naet"
	"github.com/aeternity/aepp-sdk-go/v7/transactions"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/jdgcs/ed25519/extra25519"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/nacl/box"

	shell "github.com/ipfs/go-ipfs-api"
)

var (
	cookieNameForSessionID = "mycookiesessionnameid"
	sess                   = sessions.New(sessions.Config{Cookie: cookieNameForSessionID})
)

type PageWallet struct {
	PageId       int
	PageContent  template.HTML
	PageTitle    string
	Account      string
	Balance      string
	Nonce        uint64
	Recipient_id string
	Payload      string
	Amount       string
}

var NodeOnline bool

func iRegisterNew(ctx iris.Context) {
	var myPage PageReg
	myPage.PageTitle = "Registering Page"
	myPage.SubTitle = "Decentralized knowledge system without barrier."
	myPage.Register = "Register"

	//myPage.Lang = getPageString(getPageLang(r))

	ctx.ViewData("", myPage)
	ctx.View("register.php")
}

func iImportUI(ctx iris.Context) {
	ctx.View("import.php")
}
func iExportFromMnemonic(ctx iris.Context) {

	var curve25519Private []byte
	var recipientPrivateKeySlice [64]byte

	entropy := globalAccount.SigningKey
	copy(recipientPrivateKeySlice[0:64], entropy[0:64])
	myrecipientPrivateKey := &recipientPrivateKeySlice

	_, recipientPrivateKey, err := box.GenerateKey(crypto_rand.Reader)
	if err != nil {
		panic(err)
	}

	extra25519.PrivateKeyToCurve25519(recipientPrivateKey, myrecipientPrivateKey)
	curve25519Private = recipientPrivateKey[:]
	fmt.Println(hex.EncodeToString(curve25519Private))
	fromHex, _ := hex.DecodeString(hex.EncodeToString(curve25519Private))
	mnemomic, _ := bip39.NewMnemonic(fromHex)
	fmt.Println(mnemomic)

	/*entropy := globalAccount.SigningKey

	fmt.Println(hex.EncodeToString(globalAccount.SigningKey))

	var recipientPrivateKeySlice [64]byte
	copy(recipientPrivateKeySlice[0:64], entropy[0:64])

	myrecipientPrivateKey := &recipientPrivateKeySlice

	extra25519.PrivateKeyToCurve25519(recipientPrivateKey, myrecipientPrivateKey)

	mypkk := myrecipientPrivateKey
	mnemomic, _ := bip39.NewMnemonic(mypkk)
	fmt.Println(mnemomic)
	//fmt.Println(string(mypkk))
	//ctx.View("import.php")*/

}

func iImportFromMnemonic(ctx iris.Context) {
	password := ctx.FormValue("password")
	password_repeat := ctx.FormValue("password_repeat")
	mnemonic := ctx.FormValue("mnemonic")
	account_index, _ := strconv.ParseInt(ctx.FormValue("account_index"), 10, 32)
	address_index, _ := strconv.ParseInt(ctx.FormValue("address_index"), 10, 32)

	if (password == password_repeat) && len(password) > 1 {
		seed, err := account.ParseMnemonic(mnemonic)
		if err != nil {
			fmt.Println(err)
		}

		// Derive the subaccount m/44'/457'/3'/0'/1'
		key, err := account.DerivePathFromSeed(seed, uint32(account_index), uint32(address_index))
		if err != nil {
			fmt.Println(err)
		}

		// Deriving the aeternity Account from a BIP32 Key is a destructive process
		mykey, err := account.BIP32KeyToAeKey(key)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(mykey.Address)

		jks, err := account.KeystoreSeal(mykey, password)
		alias := "Import"
		//check the database
		db, err := sql.Open("sqlite3", "./data/accounts/accounts.db")
		checkError(err)

		sql_account := "SELECT account,alias FROM accounts WHERE account='" + mykey.Address + "'"
		rows, err := db.Query(sql_account)
		checkError(err)

		needStore := true
		for rows.Next() {
			needStore = false
		}

		mnemonic = SealMSGTo(mykey.Address, mnemonic, mykey) //Crypt mnemonic
		//Store the account and initial the enviroment:config,pubdata,privatedata and logs
		if needStore {
			sql_insert := "INSERT INTO accounts(account,alias,keystore,mnemonic) VALUES ('" + mykey.Address + "','" + alias + "','" + string(jks) + "','" + mnemonic + "')"
			db.Exec(sql_insert)
			db.Close()
			//Create database for each new account
			InitDatabase(mykey.Address, alias)
		} else {
			ctx.HTML("<h1>Account Exist</h1>")
		}

		db.Close()

		ctx.Redirect("/")
	} else {
		ctx.HTML("<h1>Passwords must be the same.</h1>")
	}
}

func iDoRegister(ctx iris.Context) {
	password := ctx.FormValue("password")
	password_repeat := ctx.FormValue("password_repeat")
	alias := ctx.FormValue("alias")
	if (password == password_repeat) && len(password) > 1 {

		//Gnerate new account's mnemonic
		entropy, _ := bip39.NewEntropy(256)
		mnemonic, _ := bip39.NewMnemonic(entropy)
		seed, err := account.ParseMnemonic(mnemonic)

		// Derive the subaccount m/44'/457'/3'/0'/1'
		key, err := account.DerivePathFromSeed(seed, 0, 0)
		if err != nil {
			fmt.Println(err)
		}

		// Deriving the aeternity Account from a BIP32 Key is a destructive process
		mykey, err := account.BIP32KeyToAeKey(key)
		if err != nil {
			fmt.Println(err)
		}

		jks, err := account.KeystoreSeal(mykey, password)
		//fmt.Println(string(jks), alias)

		//check the database
		db, err := sql.Open("sqlite3", "./data/accounts/accounts.db")
		checkError(err)

		sql_account := "SELECT account,alias FROM accounts WHERE account='" + mykey.Address + "'"
		rows, err := db.Query(sql_account)
		checkError(err)

		needStore := true
		for rows.Next() {
			needStore = false
		}

		mnemonic = SealMSGTo(mykey.Address, mnemonic, mykey) //Crypt mnemonic
		//Store the account and initial the enviroment:config,pubdata,privatedata and logs
		if needStore {
			sql_insert := "INSERT INTO accounts(account,alias,keystore,mnemonic) VALUES ('" + mykey.Address + "','" + alias + "','" + string(jks) + "','" + mnemonic + "')"
			db.Exec(sql_insert)
			db.Close()
			//Create database for each new account
			InitDatabase(mykey.Address, alias)
		}

		ctx.Redirect("/")
	} else {
		ctx.HTML("<h1>Passwords must be the same.</h1>")
	}
}

func iDoRegister_old(ctx iris.Context) {
	password := ctx.FormValue("password")
	password_repeat := ctx.FormValue("password_repeat")
	//alias := ctx.FormValue("alias")
	if (password == password_repeat) && len(password) > 1 {
		acc, _ := account.New()
		accountFileName := "tmpAccount"
		f, _ := account.StoreToKeyStoreFile(acc, password, accountFileName)
		//fmt.Println(acc.Address)
		//fmt.Println(f)
		newFile := "./data/accounts/" + acc.Address
		os.Rename(f, newFile)
		ctx.Redirect("/")
	} else {
		ctx.HTML("<h1>Passwords must be the same.</h1>")
	}
}
func iLogOut(ctx iris.Context) {
	//c := "killall ipfs && ./ipfs daemon"
	//_ = exec.Command("sh", "-c", c)
	//
	globalAccount.Address = ""
	session := sess.Start(ctx)
	NodeOnline = false
	loginoutFile()
	//notifyStopping()
	//aerepo.Close() //close the repo

	// Revoke users authentication
	//session.Set("authenticated", false)
	// Or to remove the variable:
	//session.Delete("authenticated")
	// Destroy the whole session:
	session.Destroy()
	ctx.Redirect("/")
	//stop current daemon
	//<-myreq.Context.Done()
	//time.Sleep(1 * time.Second)
	//go killIPFS()
}

func killIPFS() {

	if ostype == "windows" {
		c := "TASKKILL /IM ipfs.exe /F"
		fmt.Println(c)
		cmd := exec.Command("cmd", "/c", c)
		output, err := cmd.Output()

		if err != nil {
			fmt.Printf("Execute Shell:%s failed with error:%s", c, err.Error())
			return
		}
		fmt.Printf("Execute Shell:%s finished with output:\n%s", c, string(output))
	} else {
		//kill ipfs firstly
		c := `killall ipfs`
		fmt.Println(c)
		cmd := exec.Command("sh", "-c", c)
		output, err := cmd.Output()

		if err != nil {
			fmt.Printf("Execute Shell:%s failed with error:%s", c, err.Error())
			return
		}
		fmt.Printf("Execute Shell:%s finished with output:\n%s", c, string(output))
	}
}

func iHomePage(ctx iris.Context) {
	MyAENS = ""
	needReg := true
	AccountsLists := ""
	go bootIPFS()

	//Check if there is a logined account
	if len(globalAccount.Address) > 6 {
		if !checkLogin(ctx) {
			return
		}

		needReg = false

		myPage := PageWallet{PageId: 23, Account: globalAccount.Address, PageTitle: MyIPFSConfig.Identity.PeerID}
		ctx.ViewData("", myPage)
		ctx.View("dashboard.php")

		err := qrcode.WriteFile(globalAccount.Address, qrcode.Medium, 256, "./views/qr_ak.png")
		checkError(err)

	} else {
		//list accounts
		dbpath := "./data/accounts/accounts.db"
		if !FileExist(dbpath) {
			db, _ := sql.Open("sqlite3", dbpath)
			sql_account := `
CREATE TABLE if not exists "accounts"(
"aid" INTEGER PRIMARY KEY AUTOINCREMENT,
"account" TEXT NULL,
"keystore" TEXT NULL,
"alias" TEXT NULL,
"mnemonic" TEXT NULL,
"lastlogin" INTEGER NULL,
"remark" TEXT NULL
);
`
			db.Exec(sql_account)
			db.Close()
		}
		db, err := sql.Open("sqlite3", dbpath)
		checkError(err)

		sql_account := "SELECT account,alias FROM accounts ORDER by lastlogin desc"
		rows, err := db.Query(sql_account)
		checkError(err)

		for rows.Next() {
			var account string
			var alias string
			err = rows.Scan(&account, &alias)
			AccountsLists = AccountsLists + "<option value=" + account + ">" + alias + "(" + account + ")</option>\n"
			needReg = false
		}

		db.Close()

	}

	if needReg {
		var myPage PageReg
		myPage.PageTitle = "Registering Page"
		myPage.SubTitle = "Decentralized knowledge system without barrier."
		myPage.Register = "Register"

		//myPage.Lang = getPageString(getPageLang(r))

		//myPage = getPageString(getPageLang(r), "register")
		ctx.ViewData("", myPage)
		ctx.View("register.php")
	} else {
		var myoption template.HTML
		myoption = template.HTML(AccountsLists)
		myPage := PageLogin{Options: myoption}
		ctx.ViewData("", myPage)
		ctx.View("login.php")
	}

}
func iHomePage_old(ctx iris.Context) {

	needReg := true
	ak := ""
	AccountsLists := ""
	//myLang := getPageString(getPageLang(ctx.Request()))
	//language := ctx.GetLocale().Language()
	//fmt.Println(myLang.Register)
	go bootIPFS()
	merr := filepath.Walk("data/accounts/", func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, "ak_") {

			ak = filepath.Base(path)
			if len(ak) > 0 {
				AccountsLists = AccountsLists + "<option>" + ak + "</option>\n"
			}

			needReg = false
		}

		return nil
	})
	//fmt.Println("address:" + globalAccount.Address)
	if len(globalAccount.Address) > 1 {
		if !checkLogin(ctx) {
			return
		}

		needReg = false
		ak := globalAccount.Address

		myPage := PageWallet{PageId: 23, Account: ak, PageTitle: MyIPFSConfig.Identity.PeerID}
		ctx.ViewData("", myPage)
		ctx.View("dashboard.php")

		err := qrcode.WriteFile(ak, qrcode.Medium, 256, "./views/qr_ak.png")
		err = qrcode.WriteFile("https://www.aeknow.org/v2/accounts/"+ak, qrcode.Medium, 256, "./views/qr_account.png")
		if err != nil {
			fmt.Println("write error")
		}
	} else {

		var myoption template.HTML
		myoption = template.HTML(AccountsLists)
		myPage := PageLogin{Options: myoption}
		ctx.ViewData("", myPage)
		ctx.View("login.php")
	}

	if merr != nil {
		fmt.Println("error")
	}

	if needReg {

		var myPage PageReg
		myPage.PageTitle = "Registering Page"
		myPage.SubTitle = "Decentralized knowledge system without barrier."
		myPage.Register = "Register"

		//myPage.Lang = getPageString(getPageLang(r))

		//myPage = getPageString(getPageLang(r), "register")
		ctx.ViewData("", myPage)
		ctx.View("register.php")
	}
}
func iCheckLogin(ctx iris.Context) {
	accountname := ctx.FormValue("accountname")
	password := ctx.FormValue("password")
	db, err := sql.Open("sqlite3", "./data/accounts/accounts.db")
	checkError(err)

	sql_account := "SELECT keystore,alias FROM accounts WHERE account='" + accountname + "'"
	rows, err := db.Query(sql_account)
	checkError(err)

	var keystore string
	var alias string
	for rows.Next() {
		err = rows.Scan(&keystore, &alias)
		checkError(err)
	}

	//myAccount, err := account.LoadFromKeyStoreFile("data/accounts/"+accountname, password)
	myAccount, err := account.KeystoreOpen([]byte(keystore), password)
	MyUsername = alias

	if err != nil {
		fmt.Println("Could not create myAccount's Account:", err)
		myPage := PageWallet{PageTitle: "Password error:Could not Read Account"}
		ctx.ViewData("", myPage)
		ctx.View("error.php")

	} else { //init the settings
		globalAccount = *myAccount //作为呈现账号
		signAccount = myAccount    //作为签名账号

		// Set user as authenticated
		session := sess.Start(ctx)
		session.Set("authenticated", true)
		GetConfigs(myAccount.Address)
		//NodeConfig = getConfigString() //读取节点设置
		//MyIPFSConfig = getIPFSConfig() //读取IPFS节点配置
		//MySiteConfig = getSiteConfig() //读取网站设置
		lastIPFS = ""

		//go bootIPFS()
		NodeOnline = true
		loginedFile()
		go bootIPFS()
		go ConnetDefaultNodes()
		lastIPFS = getLastIPFS()
		lastlogin := strconv.FormatInt(time.Now().Unix(), 10)
		sql_update := "UPDATE accounts SET lastlogin=" + lastlogin + " WHERE account='" + myAccount.Address + "'"
		//fmt.Println(sql_update)
		db.Exec(sql_update)

	}
	db.Close()

	ctx.Redirect("/")

}
func iCheckLogin_old(ctx iris.Context) {
	accountname := ctx.FormValue("accountname")
	password := ctx.FormValue("password")
	myAccount, err := account.LoadFromKeyStoreFile("data/accounts/"+accountname, password)
	if err != nil {
		fmt.Println("Could not create myAccount's Account:", err)
		myPage := PageWallet{PageTitle: "Password error:Could not Read Account"}
		ctx.ViewData("", myPage)
		ctx.View("error.php")

	} else { //init the settings
		globalAccount = *myAccount //作为呈现账号
		signAccount = myAccount    //作为签名账号
		IPFS_PATH := "./data/site/" + globalAccount.Address + "/repo/"
		_ = os.Setenv("IPFS_PATH", IPFS_PATH)
		checkHugo()

		checkIPFSRepo(globalAccount.Address)
		// Set user as authenticated
		session := sess.Start(ctx)
		session.Set("authenticated", true)

		NodeConfig = getConfigString() //读取节点设置
		MyIPFSConfig = getIPFSConfig() //读取IPFS节点配置
		MySiteConfig = getSiteConfig() //读取网站设置
		lastIPFS = ""
		configHugo() //登录成功初始化
		//go bootIPFS()
		NodeOnline = true
		loginedFile()
		go ConnetDefaultNodes()
		lastIPFS = getLastIPFS()
		// Authentication goes here
		// ...

	}

	ctx.Redirect("/")

}

func loginedFile() {
	loginedFile := ""
	if ostype == "windows" {
		loginedFile = ".\\data\\online.lock"
	} else {
		loginedFile = "./data/online.lock"
	}

	if FileExist(loginedFile) {
	} else {
		err := ioutil.WriteFile(loginedFile, []byte("ONLINE"), 0644)
		if err != nil {
			panic(err)
		}
	}
}

func loginoutFile() {
	loginedFile := ""
	if ostype == "windows" {
		loginedFile = ".\\data\\online.lock"
	} else {
		loginedFile = "./data/online.lock"
	}

	if FileExist(loginedFile) {
		err := os.Remove(loginedFile)

		if err != nil {
			// 删除失败
			fmt.Println("logout failed")

		} else {
			// 删除成功
			fmt.Println("logout")
		}
	}
}

func bootIPFS() { //boot IPFS independently
	NeedBoot := true

	sh := shell.NewShell("127.0.0.1:5001")
	cid, err := sh.Add(strings.NewReader("Hello AEK!"))

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		//os.Exit(1)
	} else {
		fmt.Println("IPFS has booted, abort=>" + cid)
		NeedBoot = false
	}

	if NeedBoot {
		if ostype == "windows" {
			fileExec := ".\\bin\\ipfs.exe"
			//c := "set IPFS_PATH=data\\site\\" + globalAccount.Address + "\\repo\\&& " + fileExec + " daemon --enable-pubsub-experiment"
			c := "set IPFS_PATH=data\\repo\\&& " + fileExec + " daemon --enable-pubsub-experiment"
			fmt.Println(c)
			cmd := exec.Command("cmd", "/c", c)
			out, _ := cmd.Output()
			fmt.Println(string(out))

		} else {
			fileExec := "./bin/ipfs"

			//c := "export IPFS_PATH=./data/site/" + globalAccount.Address + "/repo/&& " + fileExec + " daemon --enable-pubsub-experiment"
			c := "export IPFS_PATH=./data/repo/&& " + fileExec + " daemon --enable-pubsub-experiment"
			cmd := exec.Command("sh", "-c", c)
			fmt.Println(c)
			out, _ := cmd.Output()
			fmt.Println(string(out))

		}
	}
}

func iMakeTranscaction(ctx iris.Context) {
	if !checkLogin(ctx) {
		return
	}
	sender_id := ctx.FormValue("sender_id")
	recipient_id := ctx.FormValue("recipient_id")
	amount := ctx.FormValue("amount")
	payload := ctx.FormValue("payload")
	password := ctx.FormValue("password")

	famount, err := strconv.ParseFloat(amount, 64)
	bigfloatAmount := big.NewFloat(famount)
	imultiple := big.NewFloat(1000000000000000000) //18 dec
	fmyamount := big.NewFloat(1)
	fmyamount.Mul(bigfloatAmount, imultiple)

	myamount := new(big.Int)
	fmyamount.Int(myamount)
	//transfer tokens to .chain name
	if strings.Index(recipient_id, ".chain") > -1 {
		recipient_id = getAccountFromAENS(recipient_id)
	}

	alice, err := account.LoadFromKeyStoreFile("data/accounts/"+sender_id, password)
	if err != nil {

		ak := globalAccount.Address

		myPage := PageWallet{PageId: 23, Account: ak, PageTitle: "Password error:Could not Read Account"}
		ctx.ViewData("", myPage)
		ctx.View("error.php")
		return
	}

	bobAddress := recipient_id

	// create a connection to a node, represented by *Node
	node := naet.NewNode(NodeConfig.PublicNode, false)

	// create the closures that autofill the correct account nonce and transaction TTL
	_, _, ttlnoncer := transactions.GenerateTTLNoncer(node)

	// create the SpendTransaction

	tx, err := transactions.NewSpendTx(alice.Address, bobAddress, myamount, []byte(payload), ttlnoncer)
	if err != nil {
		fmt.Println("Could not create the SpendTx:", err)
	} else {
		fmt.Println(tx)
	}

	_, myTxhash, _, _, _, err := SignBroadcastWaitTransaction(tx, alice, node, aeconfig.Node.NetworkID, 10)
	if err != nil {
		fmt.Println("SignBroadcastTransaction failed with:", err)
		ak := globalAccount.Address

		myPage := PageWallet{PageId: 23, Account: ak, PageTitle: "Failed"}
		ctx.ViewData("", myPage)
		ctx.View("transaction.php")
	} else {
		ak := globalAccount.Address
		myPage := PageWallet{PageId: 23, Account: ak, PageTitle: myTxhash}
		ctx.ViewData("", myPage)
		ctx.View("transaction.php")
	}
}

func iWallet(ctx iris.Context) {
	if !checkLogin(ctx) {
		return
	}
	needReg := true
	ak := ""
	AccountsLists := ""
	recipient_id := ""
	payload := ""
	amountstr := ""

	recipient_id = ctx.URLParam("recipient_id")
	//payloadByte = ctx.URLParam("payload")
	payloadByte, _ := base64.StdEncoding.DecodeString(ctx.URLParam("payload"))
	payload = string(payloadByte)

	amountstr = ctx.URLParam("amount")

	node := naet.NewNode(NodeConfig.PublicNode, false)

	akBalance, err := node.GetAccount(globalAccount.Address)
	var thisamount string
	var myNonce uint64
	if err != nil {
		fmt.Println("Account not exist.")
		thisamount = "0"
		myNonce = 0
	} else {
		bigstr := akBalance.Balance.String()
		myBalance := ToBigFloat(bigstr)
		imultiple := big.NewFloat(0.000000000000000001) //18 dec
		thisamount = new(big.Float).Mul(myBalance, imultiple).String()
		myNonce = *akBalance.Nonce

	}

	merr := filepath.Walk("data/accounts/", func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, "ak_") {

			ak = filepath.Base(path)
			if len(ak) > 0 {
				AccountsLists = AccountsLists + "<option>" + ak + "</option>\n"
			}

			needReg = false
		}
		//fmt.Println(path)
		return nil
	})
	//fmt.Println("address:" + globalAccount.Address)
	if len(globalAccount.Address) > 1 {
		needReg = false
		ak := globalAccount.Address

		myPage := PageWallet{PageId: 23, Account: ak, PageTitle: "Wallet", Balance: thisamount, Nonce: myNonce, Recipient_id: recipient_id, Amount: amountstr, Payload: payload}
		ctx.ViewData("", myPage)
		ctx.View("wallet.php")

		err := qrcode.WriteFile(ak, qrcode.Medium, 256, "./views/qr_ak.png")
		err = qrcode.WriteFile("https://www.aeknow.org/v2/accounts/"+ak, qrcode.Medium, 256, "./views/qr_account.png")
		if err != nil {
			fmt.Println("write error")
		}
	} else {

		var myoption template.HTML
		myoption = template.HTML(AccountsLists)
		myPage := PageLogin{Options: myoption}
		ctx.ViewData("", myPage)
		ctx.View("login.php")
	}

	if merr != nil {
		fmt.Println("error")
	}

	if needReg {

		var myPage PageReg
		myPage.PageTitle = "Registering Page"
		myPage.SubTitle = "Decentralized knowledge system without barrier."
		myPage.Register = "Register"

		myPage.Lang = getPageString(getPageLang(ctx.Request()))

		ctx.ViewData("", myPage)
		ctx.View("register.php")
	}
}
func checkHugo() {
	if ostype == "windows" {
		hugoDir := ".\\data\\site\\" + globalAccount.Address
		if !FileExist(hugoDir) {
			fileExec := "..\\..\\bin\\hugo.exe"
			c := fileExec + " new site " + globalAccount.Address
			fmt.Println(c)
			cmd := exec.Command("cmd", "/c", c)
			cmd.Dir = ".\\data\\site"
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}
			//mkdir post for hugo
			c = "md post"
			cmd = exec.Command("cmd", "/c", c)
			cmd.Dir = ".\\data\\site\\" + globalAccount.Address + "\\content"
			out, err = cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}

			//mkdir and cp theme
			c = "md data\\site\\" + globalAccount.Address + "\\themes\\aeknow"
			fmt.Println(c)
			cmd = exec.Command("cmd", "/c", c)
			out, err = cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}

			c = "xcopy /e /r /y data\\themes\\aeknow  data\\site\\" + globalAccount.Address + "\\themes\\aeknow"
			fmt.Println(c)
			cmd = exec.Command("cmd", "/c", c)
			out, err = cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}

			fmt.Println(string(out))
			//cp default site config
			c = "copy data\\site.json  data\\site\\" + globalAccount.Address + "\\"
			fmt.Println(c)
			cmd = exec.Command("cmd", "/c", c)
			out, err = cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}

			fmt.Println(string(out))
		}
	} else {
		hugoDir := "./data/site/" + globalAccount.Address
		if !FileExist(hugoDir) {
			fileExec := "../../bin/hugo"
			c := fileExec + " new site " + globalAccount.Address
			fmt.Println(c)
			cmd := exec.Command("sh", "-c", c)
			cmd.Dir = "./data/site"
			//cmd.Run()
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}
			//mkdir post for hugo
			c = "mkdir post"
			cmd = exec.Command("sh", "-c", c)
			cmd.Dir = "./data/site/" + globalAccount.Address + "/content"
			out, err = cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}
			//cp theme
			c = "cp ./data/themes/aeknow/ -r ./data/site/" + globalAccount.Address + "/themes/"
			fmt.Println(c)
			cmd = exec.Command("sh", "-c", c)
			out, err = cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println(string(out))

			//cp default site config
			c = "cp ./data/site.json ./data/site/" + globalAccount.Address + "/"
			fmt.Println(c)
			cmd = exec.Command("sh", "-c", c)
			out, err = cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println(string(out))
		}
	}
}

func configHugo() {
	if ostype == "windows" {
		//init theme config file
		themeConfigFile := readFileStr(".\\data\\themes\\config.toml")
		targetFile := ".\\data\\site\\" + globalAccount.Address + "\\config.toml"
		err := ioutil.WriteFile(targetFile, []byte(themeConfigFile), 0644)
		if err != nil {
			panic(err)
		}

		fmt.Println(targetFile + "...done.")

		//config search page
		srcFile := ".\\data\\search.html"
		targetFile = ".\\data\\site\\" + globalAccount.Address + "\\content\\search.html"
		if contents, err := ioutil.ReadFile(srcFile); err == nil {
			MyContents := strings.Replace(string(contents), "{{.PeerID}}", MyIPFSConfig.Identity.PeerID, -1)
			err := ioutil.WriteFile(targetFile, []byte(MyContents), 0644)
			if err != nil {
				panic(err)
			}
		} else {
			fmt.Println("Read search page failed")
		}

	} else {
		//init theme config file
		themeConfigFile := readFileStr("./data/themes/config.toml")
		targetFile := "./data/site/" + globalAccount.Address + "/config.toml"
		err := ioutil.WriteFile(targetFile, []byte(themeConfigFile), 0644)
		if err != nil {
			panic(err)
		}

		fmt.Println(targetFile + "...done.")
		//TODO:	1.mkdir post;2.copy themes;3.init config files;4.init ipns node info;5.add search and remove about page;6.add default help link
		//Done:
		//addstr := string(out)

		//config search page
		srcFile := "./data/search.html"
		targetFile = "./data/site/" + globalAccount.Address + "/content/search.html"
		if contents, err := ioutil.ReadFile(srcFile); err == nil {
			MyContents := strings.Replace(string(contents), "{{.PeerID}}", MyIPFSConfig.Identity.PeerID, -1)
			err := ioutil.WriteFile(targetFile, []byte(MyContents), 0644)
			if err != nil {
				panic(err)
			}
		} else {
			fmt.Println("Read search page failed")
		}

	}
}

func readFileStr(fileName string) string {
	//TODONE: how to config the site
	if contents, err := ioutil.ReadFile(fileName); err == nil {
		//因为contents是[]byte类型，直接转换成string类型后会多一行空格,需要使用strings.Replace替换换行符
		MyContents := strings.Replace(string(contents), "{{.SiteTitle}}", MySiteConfig.Title, -1)
		MyContents = strings.Replace(MyContents, "{{.Author}}", MySiteConfig.Author, -1)
		MyContents = strings.Replace(MyContents, "{{.AuthorDescription}}", MySiteConfig.AuthorDescription, -1)
		MyContents = strings.Replace(MyContents, "{{.Subtitle}}", MySiteConfig.Subtitle, -1)
		MyContents = strings.Replace(MyContents, "{{.SiteDescription}}", MySiteConfig.Description, -1)
		//get lastIPFS
		myLastIPFS := getLastIPFS()
		MyContents = strings.Replace(MyContents, "{{.LastIPFS}}", myLastIPFS, -1)

		//update user's IPFS peerid
		MyContents = strings.Replace(MyContents, "{{.Account}}", globalAccount.Address, -1)
		return strings.Replace(MyContents, "{{.Baseurl}}", NodeConfig.IPFSNode+"/ipns/"+MyIPFSConfig.Identity.PeerID+"/", -1)
		//return strings.Replace(MyContents, "{{.Baseurl}}", NodeConfig.IPFSNode+"/ipfs/"+myLastIPFS+"/", -1)

	}

	return ""
}

func getLastIPFS() string {
	dbpath := "./data/accounts/" + globalAccount.Address + "/config.db"
	db, err := sql.Open("sqlite3", dbpath)
	checkError(err)

	value := ""
	if FileExist(dbpath) {
		sql_query := "SELECT value FROM config WHERE item='LastIPFS'"
		rows, err := db.Query(sql_query)
		checkError(err)

		for rows.Next() {
			err = rows.Scan(&value)
		}
	}

	return value
}

func getLastIPFS_old() string {
	fileName := ""
	if ostype == "windows" {
		fileName = "data\\site\\" + globalAccount.Address + "\\lastIPFS"
	} else {
		fileName = "./data/site/" + globalAccount.Address + "/lastIPFS"
	}

	if FileExist(fileName) {
		fi, err := os.Open(fileName)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return ""
		}
		defer fi.Close()

		br := bufio.NewReader(fi)

		for {
			a, _, c := br.ReadLine()
			if c == io.EOF {
				break
			}
			return string(a)
		}
	} else {
		return ""
	}
	return ""
}

//TODO: Post article hash automatically

//Simple version login check for local user
func checkLogin(ctx iris.Context) bool {
	if len(globalAccount.Address) > 1 {
		return true
	}

	return false
}

func checkIPFSRepo(RepoName string) {
	IPFSCheck := "./data/site/" + RepoName + "/repo/version"

	if !FileExist(IPFSCheck) {
		if ostype == "windows" {
			IPFS_PATH := "data\\site\\" + RepoName + "\\repo"
			c := "mkdir " + IPFS_PATH + " && set IPFS_PATH=" + IPFS_PATH + "\\&& bin\\ipfs.exe init &&copy data\\swarm.key " + IPFS_PATH
			fmt.Println(c)
			cmd := exec.Command("cmd", "/c", c)
			out, _ := cmd.Output()

			fmt.Println(string(out))
		} else {
			IPFS_PATH := "./data/site/" + RepoName + "/repo"
			c := "mkdir " + IPFS_PATH + "&& export IPFS_PATH=" + IPFS_PATH + "/&& ./bin/ipfs init && cp ./data/swarm.key " + IPFS_PATH
			fmt.Println(c)
			cmd := exec.Command("sh", "-c", c)
			out, _ := cmd.Output()

			fmt.Println(string(out))

		}
	}

}

func InitDatabase(pubkey, name string) {
	dbpathDir := "./data/accounts/" + pubkey

	if !FileExist(dbpathDir) {
		err := os.Mkdir(dbpathDir, os.ModePerm)
		checkError(err)
	}

	dbpath := "file:./data/accounts/" + pubkey + "/public.db?auto_vacuum=1"
	db, _ := sql.Open("sqlite3", dbpath)
	//Create main data table for articles
	sql_main := `
CREATE TABLE if not exists "aek"(
"aid" INTEGER PRIMARY KEY AUTOINCREMENT,
"title" TEXT NULL,
"author" TEXT NULL,
"authorname" TEXT NULL,
"hash" TEXT NULL,
"abstract" TEXT NULL,
"keywords" TEXT NULL,
"filetype" TEXT NULL,
"filesize" INTEGER NULL,
"pubtime" INTEGER NULL,
"lastmodtime" INTEGER NULL,
"remark" TEXT NULL
);
`
	db.Exec(sql_main)
	//Create author's info table
	sql_site := `
CREATE TABLE if not exists "author"(
"aid" INTEGER PRIMARY KEY AUTOINCREMENT,
"pubkey" TEXT NULL,
"name" TEXT NULL,
"bio" TEXT NULL,
"aens" TEXT NULL,
"ipns" TEXT NULL,
"sitetitle" TEXT NULL,
"sitesubtitle" TEXT NULL,
"sitedescription" TEXT NULL,
"sitetheme" TEXT NULL,
"pubtime" INTEGER NULL,
"lasttime" INTEGER NULL,
"remark" TEXT NULL
);`

	db.Exec(sql_site)
	sql_init := `INSERT INTO author(pubkey,name) VALUES('` + pubkey + `','` + name + `');`
	db.Exec(sql_init)
	db.Close()

	//create private database
	dbpath = "file:./data/accounts/" + pubkey + "/private.db?auto_vacuum=1"
	db, _ = sql.Open("sqlite3", dbpath)
	sql_main = `
CREATE TABLE if not exists "aek"(
"aid" INTEGER PRIMARY KEY AUTOINCREMENT,
"title" TEXT NULL,
"author" TEXT NULL,
"authorname" TEXT NULL,
"hash" TEXT NULL,
"abstract" TEXT NULL,
"keywords" TEXT NULL,
"filetype" TEXT NULL,
"filesize" INTEGER NULL,
"pubtime" INTEGER NULL,
"lastmodtime" INTEGER NULL,
"remark" TEXT NULL
);
`
	db.Exec(sql_main)
	//Create author's info table
	sql_site = `
CREATE TABLE if not exists "author"(
"aid" INTEGER PRIMARY KEY AUTOINCREMENT,
"pubkey" TEXT NULL,
"name" TEXT NULL,
"bio" TEXT NULL,
"aens" TEXT NULL,
"ipns" TEXT NULL,
"sitetitle" TEXT NULL,
"sitesubtitle" TEXT NULL,
"sitedescription" TEXT NULL,
"sitetheme" TEXT NULL,
"pubtime" INTEGER NULL,
"lasttime" INTEGER NULL,
"remark" TEXT NULL
);`

	db.Exec(sql_site)
	sql_init = `INSERT INTO author(pubkey,name) VALUES('` + pubkey + `','` + name + `');`
	db.Exec(sql_init)
	db.Close()

	//create config database
	dbpath = "file:./data/accounts/" + pubkey + "/config.db?auto_vacuum=1"
	db, _ = sql.Open("sqlite3", dbpath)

	sql_table := `
CREATE TABLE if not exists "config"(
"item" text NULL,
"value" TEXT NULL,
"remark" TEXT NULL
);
`
	db.Exec(sql_table)
	//Default settings
	sql_init = `INSERT INTO config(item,value) VALUES('PublicNode','http://111.231.110.42:3013');`
	db.Exec(sql_init)

	sql_init = `INSERT INTO config(item,value) VALUES('APINode','https://www.aeknow.org');`
	db.Exec(sql_init)

	sql_init = `INSERT INTO config(item,value) VALUES('IPFSNode','http://127.0.0.1:8080');`
	db.Exec(sql_init)

	sql_init = `INSERT INTO config(item,value) VALUES('IPFSAPI','http://127.0.0.1:5001');`
	db.Exec(sql_init)

	sql_init = `INSERT INTO config(item,value) VALUES('LocalWeb','http://127.0.0.1:8888');`
	db.Exec(sql_init)
	db.Close()

	//create logs database
	dbpath = "file:./data/accounts/" + pubkey + "/logs.db?auto_vacuum=1"
	db, _ = sql.Open("sqlite3", dbpath)
	//Create log data table for articles
	sql_main = `
CREATE TABLE if not exists "logs"(
"aid" INTEGER PRIMARY KEY AUTOINCREMENT,
"title" TEXT NULL,
"author" TEXT NULL,
"authorname" TEXT NULL,
"hash" TEXT NULL,
"abstract" TEXT NULL,
"keywords" TEXT NULL,
"filetype" TEXT NULL,
"filesize" INTEGER NULL,
"pubtime" INTEGER NULL,
"lastmodtime" INTEGER NULL,
"remark" TEXT NULL
);
`
	db.Exec(sql_main)

	//create indexs for search
	sql_index := `create index bodyindex on aek(body);`
	db.Exec(sql_index)

	sql_index = `create index titleindex on aek(title);`
	db.Exec(sql_index)

	sql_index = `create index keywordsindex on aek(keywords);`
	db.Exec(sql_index)

	sql_index = `create index authorindex on aek(author);`
	db.Exec(sql_index)

	db.Close()

}
func UpdateConfigs(pubkey, item, value string) {
	//update or insert configs
	dbpath := "./data/accounts/" + pubkey + "/config.db"
	db, err := sql.Open("sqlite3", dbpath)
	checkError(err)
	sql_check := "SELECT value FROM config WHERE item='" + item + "'"
	rows, err := db.Query(sql_check)
	checkError(err)

	NeedInsert := true
	for rows.Next() {
		NeedInsert = false
	}

	if NeedInsert {
		sql_insert := "INSERT INTO config(item,value) VALUES('" + item + "','" + value + "')"
		db.Exec(sql_insert)
	} else {
		sql_update := "UPDATE config set value='" + value + "' WHERE item='" + item + "'"
		db.Exec(sql_update)
	}

	db.Close()
}
func GetConfigs(pubkey string) {
	dbpath := "./data/accounts/" + pubkey + "/config.db"
	db, _ := sql.Open("sqlite3", dbpath)
	sql_query := "SELECT item, value FROM config"
	rows, err := db.Query(sql_query)
	checkError(err)

	MySiteConfig.Author = pubkey
	MySiteConfig.AuthorDescription = pubkey
	MySiteConfig.Description = "This is the default new site, ready to build my knowledge base!"
	MySiteConfig.Title = "New Start"
	MySiteConfig.Subtitle = "A new step to the knowledge ocean."

	for rows.Next() {
		var item string
		var value string
		err = rows.Scan(&item, &value)
		switch item {
		//site config
		case "Author":
			MySiteConfig.Author = value
		case "Title":
			MySiteConfig.Title = value
		case "AuthorDescription":
			MySiteConfig.AuthorDescription = value
		case "Description":
			MySiteConfig.Description = value
		case "Subtitle":
			MySiteConfig.Subtitle = value
		//network config
		case "PublicNode":
			NodeConfig.PublicNode = value
		case "APINode":
			NodeConfig.APINode = value
		case "IPFSNode":
			NodeConfig.IPFSNode = value
		case "IPFSAPI":
			NodeConfig.IPFSAPI = value
		case "LocalWeb":
			NodeConfig.LocalWeb = value
		case "MyAENS":
			MyAENS = value
		case "LastIPFS":
			lastIPFS = value
		default:
			fmt.Println("No Such item: " + item)
		}

	}

	db.Close()
	//NodeConfig = getConfigString() //读取节点设置
	MyIPFSConfig = getIPFSConfig() //读取IPFS节点配置
	//MySiteConfig = getSiteConfig() //读取网站设置
}
