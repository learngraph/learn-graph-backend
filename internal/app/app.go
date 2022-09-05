package app

import "log"

func Main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	runGQLServer()
}
