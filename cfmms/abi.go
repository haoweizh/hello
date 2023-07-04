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
	cmd := exec.Command("solc", "")

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
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  string `json:"result"`
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
		res, err := util.HttpRequest(http.MethodGet, "https://api-cn.etherscan.com/api?module=contract&action=getabi&address="+value.Address+"&apikey="+EtherscanApiKey, "", map[string]string{}, 30)
		if err != nil {
			log.Fatal(err)
		}
		response := Response{}
		err = json.Unmarshal([]byte(res), &response)
		if err != nil && response.Status != "1" {
			log.Fatal(err)
		}

		//保存 abi
		err = os.WriteFile(outputpath+value.Name+".abi", []byte(response.Result), 0644)
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

	// 删除文件
	err = os.RemoveAll(outputpath)
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

		//abigen --abi=/Users/uuuliu/GolandProjects/Go/hello/contracts/UniswapV3Pool/UniswapV3Pool.abi --pkg=UniswapV3Pool --out=UniswapV3Pool.go

		pwd, _ := os.Getwd()

		extension := filepath.Ext(file.Name())
		name := file.Name()[0 : len(file.Name())-len(extension)]

		// 定义要执行的命令和参数

		// 创建输出目录（如果不存在）
		err = os.MkdirAll(outputpath+name, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}

		cmd := exec.Command("abigen", "--abi="+absolutePath, "--pkg="+name, "--out="+pwd+"/"+outputpath+name+"/"+name+".go")

		//fmt.Println(cmd)
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

/*
solc --abi Store.sol
solc --bin Store.sol
abigen --bin=Store_sol_Store.bin --abi=Store_sol_Store.abi --pkg=store --out=Store.go
*/

func GenerateDeploymentGoFile(inputPath []string, outputpath string) {

	// 删除文件
	err := os.RemoveAll(outputpath)
	if err != nil {
		log.Fatal(err)
	}

	// 创建输出目录（如果不存在）
	err = os.MkdirAll(outputpath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	for _, path := range inputPath {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {

			if file.IsDir() {
				// 如果文件是目录，可以选择进行处理
				continue
			}
			filePath := filepath.Join(path, file.Name())
			// 处理文件路径
			absolutePath, err := filepath.Abs(filePath)
			if err != nil {
				log.Fatal(err)
			}

			pwd, _ := os.Getwd()

			extension := filepath.Ext(file.Name())
			name := file.Name()[0 : len(file.Name())-len(extension)]

			// 定义要执行的命令和参数

			// 创建输出目录（如果不存在）
			err = os.MkdirAll(outputpath+name, os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}

			cmd_solc_abi := exec.Command("solc", "--abi", absolutePath, "--output-dir="+pwd+"/"+outputpath+name)
			//fmt.Println(cmd)
			// 执行命令并获取输出
			_, err = cmd_solc_abi.Output()
			if err != nil {
				fmt.Println("执行cmd_solc_abi出错:", err)
				return
			}

			cmd_solc_bin := exec.Command("solc", "--bin", absolutePath, "--output-dir="+pwd+"/"+outputpath+name)
			_, err = cmd_solc_bin.Output()
			if err != nil {
				fmt.Println("执行cmd_solc_bin出错:", err)
				return
			}

			cmd_abigen_bin := exec.Command("abigen", "--bin="+pwd+"/"+outputpath+name+"/"+name+".bin", "--abi="+pwd+"/"+outputpath+name+"/"+name+".abi", "--pkg="+name, "--out="+pwd+"/"+outputpath+name+"/"+name+".go")
			_, err = cmd_abigen_bin.Output()
			fmt.Println(cmd_abigen_bin)
			if err != nil {
				fmt.Println("执行cmd_abigen_bin出错:", err)
				return
			}
		}

	}

}
