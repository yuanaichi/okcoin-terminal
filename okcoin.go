package main

import (
    "os"
    "log"
    "time"
    "encoding/json"
    "strconv"
    "runtime"
    "fmt"
    "math"
    "strings"
    
    ui "github.com/gizak/termui"

    "github.com/gorilla/websocket"
)

const (
    url = "wss://real.okcoin.cn:10440/websocket/okcoinapi"
)

type subChannel struct {
    Event   string `json:"event"`    
    Channel string `json:"channel"`
    Binary  bool   `json:"binary"`
}

type tickerData struct {
    Buy float64       `json:"buy"`
    High float64      `json:"high"`
    Low float64       `json:"low"`
    Last float64      `json:"last"`
    Sell float64      `json:"sell"`
    TimeStamp string  `json:"timestamp"`
    Vol string        `json:"vol"`
}

type dePthData struct {
    Bids [][2]float64 `json:"bids"`  //默认降序
    Asks [][2]float64 `json:"asks"`  //默认降序
    TimeStamp string  `json:"timestamp"`
}

type tradeData struct {
    //[交易序号, 价格, 成交量, 时间, 买卖类型]
    TradeNo string
    Price float64
    Vol float64
    Time string
    DealType string
}

type kindLineData struct {
    //[时间,开盘价,最高价,最低价,收盘价,成交量]
    TimeStamp float64
    OpeningPrice float64
    HighestPrice float64
    LowestPrice float64
    ClosingPrice float64
    Vol float64
}

type resMessage struct {
    Channel string   `json:"channel"`
    Data interface{} `json:"data"`
}

var par, infoPar *ui.Par
var delegateList, dealList *ui.List

var dealQueue []*tradeData

var prePrice float64

var pingChan = make(chan bool) 

var dayOpeningPrice float64

func subChannelEvent(c *websocket.Conn, channel string) {
    subChan := subChannel{Event: "addChannel", Channel: channel}
    err := c.WriteJSON(subChan)
    if err != nil {
        log.Printf("sub channel %s err :%s", channel, err)
    }
}

func sendSubChannel(c * websocket.Conn) {
    // sub channel
    subChannelEvent(c, "ok_sub_spotcny_btc_ticker")
    subChannelEvent(c, "ok_sub_spotcny_btc_depth_20")
    subChannelEvent(c, "ok_sub_spotcny_btc_trades")
    subChannelEvent(c, "ok_sub_spotcny_btc_kline_day")
}

func contains(slice []string, item string) bool {
    for _, v := range slice {
        if v == item {
            return true
        }
    }
    return false
}

func drawTicker(tData *tickerData)  {

    flag := " ︎︎"
    if prePrice == 0 {
        par.TextFgColor = ui.ColorWhite
    } else if tData.Last > prePrice {
        par.TextFgColor = ui.ColorGreen
        flag = "⬆︎︎"
    } else if tData.Last < prePrice {
        par.TextFgColor = ui.ColorRed
        flag = "⬇"
    } else {
        par.TextFgColor = ui.ColorWhite
    } 

    percentStr := " "

    if dayOpeningPrice > 0 {
        percent := (tData.Last - dayOpeningPrice) / dayOpeningPrice

        percent = percent * 100

        if percent >= 0 {
            percentStr = fmt.Sprintf("+%2.2f%%", math.Abs(percent))
        } else if percent < 0 {
            percentStr = fmt.Sprintf("-%2.2f%%", math.Abs(percent))
        }
    }
    

    par.Text = fmt.Sprintf(
        "Price: %.2f %s %s", 
        tData.Last, 
        flag,
        percentStr)

    
    infoPar.Text = fmt.Sprintf(
        "Buy 1: %.2f  Sell 1: %.2f High: %.2f Low: %.2f Vol: %s (last 24 hours)", 
        tData.Buy,
        tData.Sell,
        tData.High,
        tData.Low,
        tData.Vol)

    ui.Render(ui.Body)

    prePrice = tData.Last
}

func drawDepth(dData *dePthData)  {

    _symol := "◼"

    listNum := 5 //5档盘口

    //sell 2  10
    //sell 1  9
    //buy  1  8
    //buy  2  7
    // sell 1  buy 1 撮合

    //bids 买  1-5 降序
    var bidsItems []string
    //降序 取出前5个
    for _k, bids := range dData.Bids[:listNum] {
        // log.Printf("bids %d %f", k, bids[1])
        symolLen := int(math.Min(math.Ceil(bids[1] * 100 / 10), 20))

        bidsItems = append(
            bidsItems, 
            fmt.Sprintf("buy  %d    %.2f    %5.2f %s", _k + 1, bids[0], bids[1], strings.Repeat(_symol, symolLen)))
    }

    //asks 卖 1-5 升序   5-1 降序
    var asksItems []string
    //降序20个取出最后5个
    for _k, asks := range dData.Asks[len(dData.Asks) - listNum:] {
        // log.Printf("bids %d %f", k, bids[1])
        symolLen := int(math.Min(math.Ceil(asks[1] * 100 / 10), 20))
        asksItems = append(
            asksItems, 
            fmt.Sprintf("sell %d    %.2f    %5.2f %s", listNum - _k, asks[0], asks[1], strings.Repeat(_symol, symolLen)))
    }

    delegateItems := make([]string, len(bidsItems) + len(asksItems) + 1) //1行分割

    //合并 asksItems bidsItems 分割行
    copy(delegateItems, asksItems)
    delegateItems[len(asksItems)] = "" //分割行
    copy(delegateItems[len(asksItems) + 1:], bidsItems)

    delegateList.Items = delegateItems
    ui.Render(ui.Body)
}

func drawTrade(tData *tradeData)  {
    dealQueue = append(dealQueue, tData)

    if len(dealQueue) == 100 {
        dealQueue = dealQueue[len(dealQueue) - 10:]
    }

    dealItems := []string{}

    for i:= len(dealQueue); i > 0; i-- {
        _trade := dealQueue[i-1]
        dealItems = append(
            dealItems,
            fmt.Sprintf("%s      %.2f     %5.3f    %s", _trade.Time, _trade.Price, _trade.Vol, _trade.DealType))
    }

    dealList.Items = dealItems
    ui.Render(ui.Body)
}

func processMessage(msgChan chan []byte) {
    for message := range msgChan {
        //包含pong
        if strings.Contains(string(message), "pong") {
            //heart check 
            log.Println("receive pong message")
            pingChan <- true
            continue
        }

        var resJSON []resMessage 
        if err := json.Unmarshal(message, &resJSON); err != nil {
            log.Printf("json decode err: %s", err)
        } else {
            for _, res := range resJSON {
                if res.Data == nil {
                    continue
                }

                // ok_sub_spotcny_btc_ticker
                if res.Channel == "ok_sub_spotcny_btc_ticker" {
                    // log.Printf("data: %s", ticker.Data)
                    data := res.Data.(map[string]interface{})
                    
                    keys := []string{"buy", "sell", "high", "low", "last"}
                    tData := new(tickerData)                          
                    for k, v := range data {
                        var _v float64
                        if contains(keys, k) {
                            switch t := v.(type) {
                            case string:
                                if f, err := strconv.ParseFloat(t, 64); err == nil {
                                    // log.Printf("str to float: %f", f)
                                    _v = f
                                }
                            case float64:
                                // log.Println(v)
                                _v = t
                            }
                        }

                        switch k {
                        case "buy":
                            tData.Buy = _v
                        case "sell":
                            tData.Sell = _v
                        case "high":
                            tData.High = _v
                        case "last":
                            tData.Last = _v
                        case "low":
                            tData.Low = _v
                        case "vol":
                            tData.Vol = v.(string)
                        }
                    }

                    // log.Println(tData)
                    drawTicker(tData)
                } else if res.Channel == "ok_sub_spotcny_btc_depth_20" {
                    data := res.Data.(map[string]interface{})
                    depthData := new(dePthData)

                    for k, v := range data {
                        //k bids asks timestamp
                        switch k {
                        case "bids":
                            bids := v.([]interface{})
                            var bidsData [][2]float64
                            for _, item := range bids {
                                var _item [2]float64
                                for _k, i := range item.([]interface{}) {
                                    _item[_k] = i.(float64)
                                }
                                bidsData = append(bidsData, _item)
                            }
                            depthData.Bids = bidsData
                        case "asks":
                            asks := v.([]interface{})
                            var asksData [][2]float64
                            for _, item := range asks {
                                var _item [2]float64
                                for _k, i := range item.([]interface{}) {
                                    _item[_k] = i.(float64)
                                }
                                asksData = append(asksData, _item)
                            }
                            depthData.Asks = asksData
                        }
                    }

                    drawDepth(depthData)
                } else if res.Channel == "ok_sub_spotcny_btc_trades" {
                    data := res.Data.([]interface{})
                    
                    
                    for _, item := range data {
                        trade := item.([]interface{})

                        tData := new(tradeData)
                        tData.TradeNo = trade[0].(string)

                        var _v float64                  
                        if f, err := strconv.ParseFloat(trade[1].(string), 64); err == nil {
                            // log.Printf("str to float: %f", f)
                            _v = f
                        }

                        tData.Price = _v

                        if f, err := strconv.ParseFloat(trade[2].(string), 64); err == nil {
                            // log.Printf("str to float: %f", f)
                            _v = f
                        }

                        tData.Vol = _v

                        tData.Time = trade[3].(string)
                        tData.DealType = trade[4].(string)

                        // log.Printf("trade data %v", tData)
                        drawTrade(tData)
                    }
                } else if res.Channel == "ok_sub_spotcny_btc_kline_day" {
                    data := res.Data.([]interface{})
                    
                    log.Printf("kline data is :%#v", data)
                    
                    klData := new(kindLineData)

                    switch data[0].(type) {
                    case float64:
                        //最新数据 一条
                        klData.TimeStamp = data[0].(float64)
                        klData.OpeningPrice = data[1].(float64)
                        klData.HighestPrice = data[2].(float64)
                        klData.LowestPrice = data[3].(float64)
                        klData.ClosingPrice = data[4].(float64)
                        klData.Vol = data[5].(float64)
                    case []interface{}:
                        //第一条消息 最近几天的 日k信息
                    }
                    dayOpeningPrice = klData.OpeningPrice
                }
            }
        }
    }
}

func main() {

    //set log file
    logFileName := "btc.log"

    logFile, logErr := os.OpenFile(logFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if logErr != nil {
		fmt.Println("Fail to find", *logFile, "cServer start Failed")
		return
	}

	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)


    log.Printf("cpu num %d", runtime.NumCPU())
    runtime.GOMAXPROCS(runtime.NumCPU())

    if err := ui.Init(); err != nil {
        log.Printf("ui init err : %s", err)
        return
    }

    defer ui.Close()

    par = ui.NewPar("Last Price")
    par.Height = 3
	par.TextFgColor = ui.ColorWhite
    par.BorderLabel = "Last Price"


    infoPar = ui.NewPar("Price Info")
    infoPar.Height = 3
	infoPar.TextFgColor = ui.ColorWhite
	infoPar.BorderLabel = "Price Info"

    currentTime := time.Now().Format("2006-01-02 15:04:05")

    timePar := ui.NewPar(currentTime)
    timePar.Height = 3
	timePar.TextFgColor = ui.ColorYellow
	timePar.BorderLabel = "Current Time"

    strItems := []string{}
    delegateList = ui.NewList()
    delegateList.Items = strItems
	delegateList.ItemFgColor = ui.ColorYellow
	delegateList.BorderLabel = "Delegate List"
	delegateList.Height = 13

    dealList = ui.NewList()
    dealList.Items = strItems
	dealList.ItemFgColor = ui.ColorYellow
    dealList.Height = 13
	dealList.BorderLabel = "Deal List"

    ui.Body.AddRows(
        ui.NewRow(
            ui.NewCol(3, 0, par),
            ui.NewCol(3, 0, timePar)))

    ui.Body.AddRows(
        ui.NewRow(
            ui.NewCol(6, 0, infoPar)))

    ui.Body.AddRows(
        ui.NewRow(
            ui.NewCol(3, 0, delegateList),
            ui.NewCol(3, 0, dealList)))        

    ui.Body.Align()
    ui.Render(ui.Body)

    // websocket connect
    log.Printf("connecting to %s", url)
    c, _, err := websocket.DefaultDialer.Dial(url, nil)
    if err != nil {
        log.Fatal("dial:", err)
    }

    defer c.Close()

    // message done chan
    done := make(chan struct{})

    exit := func ()  {
        err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
        if err != nil {
            log.Printf("write close: %s", err)
        } else {
            select {
                case <-done:
                case <-time.After(time.Second):
            }

            c.Close()
        }
        ui.StopLoop()
    }

    ui.Handle("/sys/kbd/q", func (ui.Event)  {
        exit()
    })

    ui.Handle("/sys/wnd/resize", func (e ui.Event)  {
        ui.Body.Width = ui.TermWidth()
        ui.Body.Align()
        ui.Render(ui.Body)
    })

    ui.Handle("/timer/1s", func (e ui.Event)  {
        currentTime := time.Now().Format("2006-01-02 15:15:05")
        timePar.Text = currentTime
        ui.Render(ui.Body)
    })

    //----------------------- stocket message --------------

    msgChan := make(chan []byte)

    //获取消息
    go func() {
        defer c.Close()
        defer close(done)
        for {
            _, message, err := c.ReadMessage()
            if err != nil {
                log.Printf("read err: %s", err)
                return
            }
            
            msgChan <- message
        }
    }()
    
    //订阅消息
    sendSubChannel(c)

    //heart check via pingChan
    go func ()  {
        defer close(pingChan)

        for {
            if c.WriteMessage(websocket.TextMessage, []byte("{\"event\": \"ping\"}")); err != nil {
                log.Printf("send ping message err: %s", err)
                break
            }
            
            log.Println("send ping message")
            select {
            case <- pingChan:
                log.Println("server is alive")
                //收到数据 1s 后继续发送心跳
                time.Sleep(time.Second)
            case <- time.After(time.Second * 1):
                //超时 重新连接 socket
                reConnectTimes := 0 
                
                for {
                    if reConnectTimes > 5 {
                        log.Fatal("websocket connect failed")
                        break
                    }

                    _c, _, err := websocket.DefaultDialer.Dial(url, nil)
                    if err != nil {
                        log.Println("re-connect websocket faild dial:", err)
                        log.Println("after 3s retry connect websocket")
                        reConnectTimes++
                        time.Sleep(time.Second * 3)
                    } else {
                        log.Println("re-connect websocket success")
                        c = _c

                        sendSubChannel(c)
                        break
                    }
                }
            }
        }
    }()
    
    //开启10个worker 处理消息
    for i:= 0; i < 10; i ++ {
        go processMessage(msgChan)
    }

    //开始阻塞 循环terminal ui
    ui.Loop()
}