package main

import (
    "bufio"
    "fmt"
    "os/exec"
    "os"
    "strings"
    "time"
    "regexp"
    "strconv"
    "io/ioutil"
)


//thresholds&limits
var lowBitrate int64 = 1000 //low bitrate threshold (1000 bits/s)
var bitrateAgg1SecsLimit int64 = 30 //bitrate aggregation period (secs) for bitrateAgg1Avg
var bitrateAgg2SecsLimit int64 = 90 //bitrate aggregation period (secs) for bitrateAgg1Avg

//global attributes
var workDir = "/dev/shm/tsduck-stat"

//global counters
var lowBitrateSecs int64 = 0 //total seconds with low bitrate
var infoBitrateSecs int64 = 0 //total seconds with bitrate info
var bitrateAgg1Avg int64 = 0 //average bitrate (interval 1)
var bitrateAgg2Avg int64 = 0 //average bitrate (interval 2)
var missingPackets int64 = 0 //total missing TS packets
var ccErrorSeconds int64 = 0 //total seconds with CC errors


func main() {
    fmt.Println("start")

    if(len(os.Args)<2) {
        fmt.Println("please add monitoring source as a command line argument")
        os.Exit(1)
    }

    if _, err := os.Stat(workDir); os.IsNotExist(err) {
        err2 := os.Mkdir(workDir, 0755)
        if(err2 != nil) {
            fmt.Println("cannot create workdir", workDir)
            os.Exit(1)
        }
    }

    var bitrateAgg1 int64 = 0 //sum for bitrate aggregation1
    var bitrateAgg1Secs int64 = 0 //seconds for bitrate aggregation 1
    var bitrateAgg2 int64 = 0 //sum for bitrate aggregation2
    var bitrateAgg2Secs int64 = 0 //seconds for bitrate aggregation 2

    mcastGroup := os.Args[1]
    args := "--realtime -t -I ip -b 8388608 "+mcastGroup+" -O drop -P continuity -P bitrate_monitor -p 1 -t 1"
    cmd := exec.Command("tsp", strings.Split(args, " ")...)

    stderr, _ := cmd.StderrPipe()
    cmd.Start()

    //go uptimeThread()
    go writeThread(mcastGroup)

    scanner := bufio.NewScanner(stderr)
    scanner.Split(bufio.ScanLines)

    //line example:* 2019/12/28 23:41:14 - continuity: packet index: 6,078, PID: 0x0100, missing 5 packets
    //line example 2: * 2019/12/28 23:55:11 - bitrate_monitor: 2019/12/28 23:55:11, TS bitrate: 4,272,864 bits/s

    regexp1 := regexp.MustCompile(`^\*\ (.+)\ -\ ([^:]+):\ (.+)$`) //extract timestamp [1], plugin name (continuity/bitrate_monitor) [2], message [3]
    bitrateRegexp := regexp.MustCompile(`TS bitrate: ([0-9,]+) bits/s$`) //extract bitrate value
    ccRegexp := regexp.MustCompile(`missing ([0-9]+) packets$`) //extracet missing TS packets value
    missingPacketsTime := "0" //last timestamp when TS packet miss occurs

    for scanner.Scan() {
        //line := scanner.Text()
        parts := regexp1.FindStringSubmatch(scanner.Text())

        if(len(parts)!=4) {
            continue
        }
        switch parts[2] { //check plugin name
            case "continuity":
                missingArr := ccRegexp.FindStringSubmatch(parts[3])
                if(len(missingArr)!=2) {
                    continue
                }
                if(missingPacketsTime != parts[1]) { //new second with CC error
                    ccErrorSeconds++
                    missingPacketsTime = parts[1]
                }
                missingCurrent, _ := strconv.ParseInt(missingArr[1], 10, 64)
                missingPackets += missingCurrent

            case "bitrate_monitor":
                bitrateArr := bitrateRegexp.FindStringSubmatch(parts[3])
                if(len(bitrateArr)!=2) {
                    continue
                }
                bitrate, _ := strconv.ParseInt(strings.Replace(bitrateArr[1], "," ,"" ,-1), 10, 64)
                if(bitrate < lowBitrate) {
                    lowBitrateSecs++
                }

                infoBitrateSecs++

                bitrateAgg1+=bitrate //add bitrate to sum
                bitrateAgg2+=bitrate
                bitrateAgg1Secs++ //add 1 second to current agg period
                bitrateAgg2Secs++
                
                if(bitrateAgg1Secs == bitrateAgg1SecsLimit) {
                    bitrateAgg1Avg = bitrateAgg1/bitrateAgg1Secs
                    bitrateAgg1Secs=0
                    bitrateAgg1=0
                }

                if(bitrateAgg2Secs == bitrateAgg2SecsLimit) {
                    bitrateAgg2Avg = bitrateAgg2/bitrateAgg2Secs
                    bitrateAgg2Secs=0
                    bitrateAgg2=0
                }

            default:
                continue
        }
    }
    //cmd.Wait()
}

func check(e error) {
    if e != nil {
        panic(e)
    }
}

func writeThread(mcastGroup string) {
    start := time.Now() //start time for uptime calculation
    var lowBitrateSecsLast int64 = -1
    var infoBitrateSecsLast int64 = -1
    var bitrateAgg1AvgLast int64 = -1
    var bitrateAgg2AvgLast int64 = -1
    var ccErrorSecondsLast int64 = -1
    var missingPacketsLast int64 = -1

    var fileMode os.FileMode = 0644

    uptimeFile := workDir+"/uptime_"+mcastGroup
    uptimeFileTmp := workDir+"/uptime_"+mcastGroup+"_tmp"
    lowBitrateFile := workDir+"/bitr_low_secs_"+mcastGroup
    lowBitrateFileTmp := workDir+"/bitr_low_secs_"+mcastGroup+"_tmp"
    infoBitrateFile := workDir+"/bitr_info_secs_"+mcastGroup
    infoBitrateFileTmp := workDir+"/bitr_info_secs_"+mcastGroup+"_tmp"
    bitrateAgg1AvgFile := workDir+"/bitr_agg1_"+mcastGroup
    bitrateAgg1AvgFileTmp := workDir+"/bitr_agg1_"+mcastGroup+"_tmp"
    bitrateAgg2AvgFile := workDir+"/bitr_agg2_"+mcastGroup
    bitrateAgg2AvgFileTmp := workDir+"/bitr_agg2_"+mcastGroup+"_tmp"
    ccErrorSecondsFile := workDir+"/cc_secs_"+mcastGroup
    ccErrorSecondsFileTmp := workDir+"/cc_secs_"+mcastGroup+"_tmp"
    missingPacketsFile := workDir+"/cc_miss_"+mcastGroup
    missingPacketsFileTmp := workDir+"/cc_miss_"+mcastGroup+"_tmp"


    for {
        uptime := time.Since(start).Nanoseconds()/1e6
        uptimeString := strconv.FormatInt(uptime, 10)
        
        err := ioutil.WriteFile(uptimeFileTmp, []byte(uptimeString), fileMode)
        check(err)
        err2 := os.Rename(uptimeFileTmp, uptimeFile)
        check(err2)

        if(ccErrorSecondsLast != ccErrorSeconds) {
            ccErrorSecondsWr := ccErrorSeconds
            err11 := ioutil.WriteFile(ccErrorSecondsFileTmp, []byte(strconv.FormatInt(ccErrorSecondsWr*1000, 10)), fileMode)
            check(err11)
            err12 := os.Rename(ccErrorSecondsFileTmp, ccErrorSecondsFile)
            check(err12)
            ccErrorSecondsLast=ccErrorSecondsWr
        }

        if(missingPacketsLast != missingPackets) {
            missingPacketsWr := missingPackets
            err9 := ioutil.WriteFile(missingPacketsFileTmp, []byte(strconv.FormatInt(missingPacketsWr, 10)), fileMode)
            check(err9)
            err10 := os.Rename(missingPacketsFileTmp, missingPacketsFile)
            check(err10)
            missingPacketsLast=missingPacketsWr
        }

        if(lowBitrateSecsLast != lowBitrateSecs) {
            lowBitrateSecsWr := lowBitrateSecs
            err3 := ioutil.WriteFile(lowBitrateFileTmp, []byte(strconv.FormatInt(lowBitrateSecsWr*1000, 10)), fileMode)
            check(err3)
            err4 := os.Rename(lowBitrateFileTmp, lowBitrateFile)
            check(err4)
            lowBitrateSecsLast=lowBitrateSecsWr
        }

        if(infoBitrateSecsLast != infoBitrateSecs) {
            infoBitrateSecsWr := infoBitrateSecs
            err13 := ioutil.WriteFile(infoBitrateFileTmp, []byte(strconv.FormatInt(infoBitrateSecsWr*1000, 10)), fileMode)
            check(err13)
            err14 := os.Rename(infoBitrateFileTmp, infoBitrateFile)
            check(err14)
            infoBitrateSecsLast=infoBitrateSecsWr
        }

        if(bitrateAgg1AvgLast != bitrateAgg1Avg) {
            bitrateAgg1AvgWr := bitrateAgg1Avg
            err5 := ioutil.WriteFile(bitrateAgg1AvgFileTmp, []byte(strconv.FormatInt(bitrateAgg1AvgWr, 10)), fileMode)
            check(err5)
            err6 := os.Rename(bitrateAgg1AvgFileTmp, bitrateAgg1AvgFile)
            check(err6)
            bitrateAgg1AvgLast=bitrateAgg1AvgWr
        }

        if(bitrateAgg2AvgLast != bitrateAgg2Avg) {
            bitrateAgg2AvgWr := bitrateAgg2Avg
            err7 := ioutil.WriteFile(bitrateAgg2AvgFileTmp, []byte(strconv.FormatInt(bitrateAgg2AvgWr, 10)), fileMode)
            check(err7)
            err8 := os.Rename(bitrateAgg2AvgFileTmp, bitrateAgg2AvgFile)
            check(err8)
            bitrateAgg2AvgLast=bitrateAgg2AvgWr
        }


        time.Sleep(time.Second)
    }
}
