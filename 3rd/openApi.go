package beats

import (
	"fmt"
	"os"
	"runtime"

	"github.com/elastic/beats/v7/winlogbeat/cmd"
)

func RunlogBeat() {

	tmp := os.Args[0:]
	os.Args = tmp[0:1]

	defer func() {
		os.Args = tmp[0:]
	}()

	if runtime.GOOS == "windows" {
		fmt.Println("************************** Winlogbeat **************************")
		if err := cmd.RootCmd.Execute(); err != nil {
			fmt.Println("Load winlogbeat FAIL")
			os.Args = tmp[0:]
			os.Exit(1)
		}
		fmt.Println("Load winlogbeat successfully")
		return
	} else if runtime.GOOS == "linux" {
		fmt.Println("************************** filebeat **************************")
		return
	}
}
