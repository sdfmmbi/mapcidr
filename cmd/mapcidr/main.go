package main

import (
	"bufio"
	"flag"
	"net"
	"os"
	"sync"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/mapcidr"
)

// Options contains cli options
type Options struct {
	Slices    int
	HostCount int
	Cidr      string
	FileCidr  string
	Silent    bool
	Version   bool
	Output    string
	// NoColor   bool
	// Verbose   bool
}

const banner = `
                   ____________  ___    
  __ _  ___ ____  / ___/  _/ _ \/ _ \   
 /  ' \/ _ '/ _ \/ /___/ // // / , _/   
/_/_/_/\_,_/ .__/\___/___/____/_/|_| v0.0.2
          /_/                                                     	 
`

// Version is the current version of mapcidr
const Version = `0.0.2`

// showBanner is used to show the banner to the user
func showBanner() {
	gologger.Printf("%s\n", banner)
	gologger.Printf("\t\tprojectdiscovery.io\n\n")

	gologger.Labelf("Use with caution. You are responsible for your actions\n")
	gologger.Labelf("Developers assume no liability and are not responsible for any misuse or damage.\n")
}

// ParseOptions parses the command line options for application
func ParseOptions() *Options {
	options := &Options{}

	flag.IntVar(&options.Slices, "sbc", 0, "Slice by CIDR count")
	flag.IntVar(&options.HostCount, "sbh", 0, "Slice by HOST count")
	flag.StringVar(&options.Cidr, "cidr", "", "Single CIDR to process")
	flag.StringVar(&options.FileCidr, "l", "", "File containing CIDR")
	flag.StringVar(&options.Output, "o", "", "File to write output to (optional)")
	flag.BoolVar(&options.Silent, "silent", false, "Silent mode")
	flag.BoolVar(&options.Version, "version", false, "Show version")
	flag.Parse()

	// Read the inputs and configure the logging
	options.configureOutput()

	showBanner()

	if options.Version {
		gologger.Infof("Current Version: %s\n", Version)
		os.Exit(0)
	}

	options.validateOptions()

	return options
}
func (options *Options) validateOptions() {
	if options.Cidr == "" && !hasStdin() && options.FileCidr == "" {
		gologger.Fatalf("No input provided!\n")
	}

	if options.Slices > 0 && options.HostCount > 0 {
		gologger.Fatalf("sbc and sbh cant be used together!\n")
	}

	if options.Cidr != "" && options.FileCidr != "" {
		gologger.Fatalf("CIDR and List input cant be used together!\n")
	}
}

// configureOutput configures the output on the screen
func (options *Options) configureOutput() {
	// If the user desires verbose output, show verbose output
	// if options.Verbose {
	// 	gologger.MaxLevel = gologger.Verbose
	// }
	// if options.NoColor {
	// 	gologger.UseColors = false
	// }
	if options.Silent {
		gologger.MaxLevel = gologger.Silent
	}
}

var options *Options

func main() {
	options = ParseOptions()
	chancidr := make(chan string)
	outputchan := make(chan string)
	var wg sync.WaitGroup

	wg.Add(1)
	go process(&wg, chancidr, outputchan)
	wg.Add(1)
	go output(&wg, outputchan)

	if options.Cidr != "" {
		chancidr <- options.Cidr
	}

	if hasStdin() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			chancidr <- scanner.Text()
		}
	}

	if options.FileCidr != "" {
		file, err := os.Open(options.FileCidr)
		if err != nil {
			gologger.Fatalf("%s\n", err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			chancidr <- scanner.Text()
		}
	}

	close(chancidr)

	wg.Wait()
}

func process(wg *sync.WaitGroup, chancidr, outputchan chan string) {
	defer wg.Done()
	for cidr := range chancidr {
		// test if we have a cidr
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			gologger.Fatalf("%s\n", err)
		}
		if options.Slices > 0 {
			subnets, err := mapcidr.SplitN(cidr, options.Slices)
			if err != nil {
				gologger.Fatalf("%s\n", err)
			}
			for _, subnet := range subnets {
				outputchan <- subnet.String()
			}
		} else if options.HostCount > 0 {
			subnets, err := mapcidr.SplitByNumber(cidr, options.HostCount)
			if err != nil {
				gologger.Fatalf("%s\n", err)
			}
			for _, subnet := range subnets {
				outputchan <- subnet.String()
			}
		} else {
			ips, err := mapcidr.IPAddresses(cidr)
			if err != nil {
				gologger.Fatalf("%s\n", err)
			}
			for _, ip := range ips {
				outputchan <- ip
			}
		}
	}

	close(outputchan)
}

func output(wg *sync.WaitGroup, outputchan chan string) {
	defer wg.Done()

	var f *os.File
	if options.Output != "" {
		var err error
		f, err = os.Create(options.Output)
		if err != nil {
			gologger.Fatalf("Could not create output file '%s': %s\n", options.Output, err)
		}
		defer f.Close()
	}
	for o := range outputchan {
		if o == "" {
			continue
		}
		gologger.Silentf("%s\n", o)
		if f != nil {
			f.WriteString(o + "\n")
		}
	}
}

func hasStdin() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeNamedPipe == 0 {
		return false
	}
	return true
}
