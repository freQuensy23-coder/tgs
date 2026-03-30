package cli

import "fmt"

func printUsage() {
	fmt.Print(`tgs - send files to Telegram

Usage:
  tgs <file>                  Send file to Saved Messages
  tgs <folder>                Send folder as zip to Saved Messages
  tgs <user> <file|folder>    Send to specific user/chat
  tgs login bot               Setup bot authentication
  tgs login user              Setup user authentication

Config: ~/.tgs/
`)
}
