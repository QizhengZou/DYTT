/*
 * @Author: jf-011101 2838264218@qq.com
 * @Date: 2022-08-19 18:42:33
 * @LastEditors: jf-011101 2838264218@qq.com
 * @LastEditTime: 2022-08-19 18:42:57
 * @FilePath: \dytt\cmd\api\discover\instance.go
 * @Description: functions and args
 */
package discovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/resolver"
)

type Server struct {
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	Version string `json:"version"`
	Weight  int64  `json:"weight"`
}

func BuildPrefix(server Server) string {
	if server.Version == "" {
		return fmt.Sprintf("/%s/", server.Name)
	}

	return fmt.Sprintf("/%s/%s/", server.Name, server.Version)
}

func BuildRegisterPath(server Server) string {
	return fmt.Sprintf("%s%s", BuildPrefix(server), server.Addr)
}

// ParseValue unmarshall a value to a server
func ParseValue(value []byte) (Server, error) {
	server := Server{}
	if err := json.Unmarshal(value, &server); err != nil {
		return server, err
	}

	return server, nil
}

func SplitPath(path string) (Server, error) {
	server := Server{}
	strs := strings.Split(path, "/")
	if len(strs) == 0 {
		return server, errors.New("invalid path")
	}

	server.Addr = strs[len(strs)-1]

	return server, nil
}

// Exist could judge whether a service exists
func Exist(l []resolver.Address, addr resolver.Address) bool {
	for i := range l {
		if l[i].Addr == addr.Addr {
			return true
		}
	}

	return false
}

func Remove(s []resolver.Address, addr resolver.Address) ([]resolver.Address, bool) {
	for i := range s {
		if s[i].Addr == addr.Addr {
			s[i] = s[len(s)-1]
			return s[:len(s)-1], true
		}
	}
	return nil, false
}

func BuildResolverUrl(app string) string {
	return schema + ":///" + app
}
