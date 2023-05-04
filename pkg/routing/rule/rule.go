package rule

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"regexp"
	"sync"
	"unicode/utf8"

	"github.com/oschwald/geoip2-golang"
	"github.com/robertkrimen/otto"
)

var (
	logger = log.New(os.Stdout, "rule: ", log.LstdFlags|log.Lshortfile)
)

func init() {
	logger.Println("init rule")
}

type Rule struct {
	// rule name
	Name string
	// rule script
	Scripts string
	vm      *otto.Otto
	db      *geoip2.Reader
	mux     sync.Mutex
}

func New(path string, geoip string) *Rule {
	// init otto

	scripts, err := ioutil.ReadFile(path)
	if err != nil {
		logger.Fatalln("read rule file error: ", err)
		return nil
	}

	// run script

	vm := otto.New()

	var db *geoip2.Reader = nil
	if geoip != "" {
		db, err = geoip2.Open(geoip)
		if err != nil {
			logger.Fatal(err)
			return nil
		}
	}

	logger.Println("init rule vm")
	// register function

	return &Rule{
		Name:    "main_rule",
		Scripts: string(scripts),
		vm:      vm,
		db:      db,
	}
}

func (r *Rule) Run() error {
	// run script
	_, err := r.vm.Run(r.Scripts)
	if err != nil {
		logger.Println("run rule script error: ", err)
		return err
	}

	if r.db == nil {
		return nil
	}

	db := r.db
	vm := r.vm
	// register geoip function to script
	r.vm.Set("getCountry", func(call otto.FunctionCall) otto.Value {
		ip := net.ParseIP(call.Argument(0).String())
		record, err := db.City(ip)
		if err != nil {
			logger.Println(err)
			return otto.Value{}
		}
		country := record.Country.IsoCode

		ret, err := vm.ToValue(country)
		if err != nil {
			logger.Println(err)
			return otto.Value{}
		}

		return ret
	})

	return nil
}

func checkDomain(name string) error {
	switch {
	case len(name) == 0:
		return nil // an empty domain name will result in a cookie without a domain restriction
	case len(name) > 255:
		return fmt.Errorf("cookie domain: name length is %d, can't exceed 255", len(name))
	}
	var l int
	for i := 0; i < len(name); i++ {
		b := name[i]
		if b == '.' {
			// check domain labels validity
			switch {
			case i == l:
				return fmt.Errorf("cookie domain: invalid character '%c' at offset %d: label can't begin with a period", b, i)
			case i-l > 63:
				return fmt.Errorf("cookie domain: byte length of label '%s' is %d, can't exceed 63", name[l:i], i-l)
			case name[l] == '-':
				return fmt.Errorf("cookie domain: label '%s' at offset %d begins with a hyphen", name[l:i], l)
			case name[i-1] == '-':
				return fmt.Errorf("cookie domain: label '%s' at offset %d ends with a hyphen", name[l:i], l)
			}
			l = i + 1
			continue
		}
		// test label character validity, note: tests are ordered by decreasing validity frequency
		if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-' || b >= 'A' && b <= 'Z') {
			// show the printable unicode character starting at byte offset i
			c, _ := utf8.DecodeRuneInString(name[i:])
			if c == utf8.RuneError {
				return fmt.Errorf("cookie domain: invalid rune at offset %d", i)
			}
			return fmt.Errorf("cookie domain: invalid character '%c' at offset %d", c, i)
		}
	}
	// check top level domain validity
	switch {
	case l == len(name):
		return fmt.Errorf("cookie domain: missing top level domain, domain can't end with a period")
	case len(name)-l > 63:
		return fmt.Errorf("cookie domain: byte length of top level domain '%s' is %d, can't exceed 63", name[l:], len(name)-l)
	case name[l] == '-':
		return fmt.Errorf("cookie domain: top level domain '%s' at offset %d begins with a hyphen", name[l:], l)
	case name[len(name)-1] == '-':
		return fmt.Errorf("cookie domain: top level domain '%s' at offset %d ends with a hyphen", name[l:], l)
	case name[l] >= '0' && name[l] <= '9':
		return fmt.Errorf("cookie domain: top level domain '%s' at offset %d begins with a digit", name[l:], l)
	}
	return nil
}

// how to check string is ipaddr with regexp?
func checkValid(ipaddr string) bool {
	if checkDomain(ipaddr) == nil {
		return true
	}
	// check
	ret, err := regexp.Match(`^(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])\.(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])\.(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])\.(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])$`, []byte(ipaddr))
	if err != nil {
		logger.Println("check ipaddr error: ", err)
		return false
	}

	return ret
}

// match ip or domain
func (r *Rule) MatchDomain(domain string) bool {
	r.mux.Lock()
	defer r.mux.Unlock()

	if !checkValid(domain) {
		logger.Println("domain not valid")
		return false
	}

	val_bool, err := r.vm.Run("matchDomain('" + domain + "')")
	if err != nil {
		logger.Println("run matchDomain error: ", err)
		return false
	}

	ret, err := val_bool.ToBoolean()
	if err != nil {
		logger.Println("matchDomain return not boolean")
		return false
	}
	return ret
}
