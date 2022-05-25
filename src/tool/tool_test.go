package tool

import (
	"fmt"
	"testing"
)

func TestTempTest(t *testing.T) {
	TempTest()
}

func TestPathSeparatorConvert(t *testing.T) {
	fmt.Print(PathSeparatorConvert("/home/abc/qjh.txt"))
}

func TestListDir(t *testing.T) {

	list, dircount, errlist := ListDir("F:\\a\\qjh")
	fmt.Print(errlist)
	fmt.Printf("dircount=%d,count=%d\n", dircount, len(list))
	if len(list) < 300 {
		fmt.Printf("%#v\n", list)
	}
}

func TestListDirWithString(t *testing.T) {
	withString, dircount, err := ListDirWithString("f:\\a")
	fmt.Printf("%s,%d,%v\n", withString, dircount, err)
}

func TestCreateUUID(t *testing.T) {
	CreateUUID()
}

func TestFormatPath(t *testing.T) {
	fmt.Printf("%s,%s\n", "../", FormatPath("../"))
	fmt.Printf("%s,%s\n", "./", FormatPath("./"))
	fmt.Printf("%s,%s\n", "/", FormatPath("/"))
	fmt.Printf("%s,%s\n", "../server/", FormatPath("../server/"))
	fmt.Printf("%s,%s\n", "../server/", FormatPath("./server/"))
}
