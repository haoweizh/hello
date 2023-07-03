package cfmms

import (
	"encoding/json"
	"fmt"
	"hello/util"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	EtherscanApiKey = "1K75KH2KUU5GCXJKKUV19U4WQ85TNUUM9M"
)

func Cmd() {
	// 定义要执行的命令和参数
	cmd := exec.Command("ls", "-l")

	// 执行命令并获取输出
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("执行命令出错:", err)
		return
	}

	// 输出命令执行结果
	fmt.Println(string(output))
}

type Contract struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type ContractData struct {
	Data []Contract `json:"data"`
}

type Response struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Result  []ResultDetail `json:"result"`
}

type ResultDetail struct {
	SourceCode           string `json:"SourceCode"`
	ABI                  string `json:"ABI"`
	ContractName         string `json:"ContractName"`
	CompilerVersion      string `json:"CompilerVersion"`
	OptimizationUsed     string `json:"OptimizationUsed"`
	Runs                 string `json:"Runs"`
	ConstructorArguments string `json:"ConstructorArguments"`
	EVMVersion           string `json:"EVMVersion"`
	Library              string `json:"Library"`
	LicenseType          string `json:"LicenseType"`
	Proxy                string `json:"Proxy"`
	Implementation       string `json:"Implementation"`
	SwarmSource          string `json:"SwarmSource"`
}

func GetAbiFromEtherscan(inputpath string, outputpath string) {
	// 读取JSON文件
	data, err := ioutil.ReadFile(inputpath)
	if err != nil {
		log.Fatal(err)
	}

	// 解析JSON数据
	var contractdata ContractData
	err = json.Unmarshal(data, &contractdata)
	if err != nil {
		log.Fatal(err)
	}

	// 删除文件
	err = os.RemoveAll(outputpath)
	if err != nil {
		log.Fatal(err)
	}

	// 创建输出目录（如果不存在）
	err = os.MkdirAll(outputpath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	for _, value := range contractdata.Data {
		res, err := util.HttpRequest(http.MethodGet, "https://api-cn.etherscan.com/api?module=contract&action=getsourcecode&address="+value.Address+"&apikey="+EtherscanApiKey, "", map[string]string{}, 30)
		if err != nil {
			log.Fatal(err)
		}
		response := Response{}
		err = json.Unmarshal([]byte(res), &response)
		if err != nil && response.Status != "1" {
			log.Fatal(err)
		}

		//保存 abi

		// 设置输出目录和文件名
		outputPath := fmt.Sprintf("%s/%s", outputpath, value.Name+".abi")
		// 创建用于写入的文件
		file, err := os.Create(outputPath)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		// 将数据编码为JSON并写入文件
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		fmt.Println(response.Result)
		err = encoder.Encode(response.Result[0].ABI)
		if err != nil {
			log.Fatal(err)
		}

		// 保存 sol

		// 设置输出目录和文件名
		outputSolPath := fmt.Sprintf("%s/%s", outputpath, value.Name+".sol")
		// 创建用于写入的文件
		fileSol, err := os.Create(outputSolPath)
		if err != nil {
			log.Fatal(err)
		}
		defer fileSol.Close()

		err = os.WriteFile(outputSolPath, []byte(response.Result[0].SourceCode), 0644)
		if err != nil {
			fmt.Println("写入文件时发生错误:", err)
			return
		}

	}

}

func GenerateGoFilesByAbi(inputpath string, outputpath string) {

	files, err := ioutil.ReadDir(inputpath)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() {
			// 如果文件是目录，可以选择进行处理
			continue
		}
		filePath := filepath.Join(inputpath, file.Name())
		// 处理文件路径
		fmt.Println(filePath)
		absolutePath, err := filepath.Abs(filePath)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("绝对路径:", absolutePath)

		//abigen --abi=/Users/uuuliu/GolandProjects/Go/hello/contracts/UniswapV3Pool/UniswapV3Pool.abi --pkg=UniswapV3Pool --out=UniswapV3Pool.go

		// 定义要执行的命令和参数
		cmd := exec.Command("", "-l")

		// 执行命令并获取输出
		output, err := cmd.Output()
		if err != nil {
			fmt.Println("执行命令出错:", err)
			return
		}

		// 输出命令执行结果
		fmt.Println(string(output))

	}
}
