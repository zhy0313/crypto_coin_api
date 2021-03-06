package okcoin

import
(
	. "../"
	"strconv"
	"errors"
	"net/http"
	"net/url"
	"encoding/json"
	"strings"
	"io/ioutil"
	"fmt"
)

const
(
	EXCHANGE_NAME = "okcoin_cn";
	api_base_url = "https://www.okcoin.cn/api/v1/";
	url_ticker = "https://www.okcoin.cn/api/v1/ticker.do";
	url_depth = "https://www.okcoin.cn/api/v1/depth.do";
	url_trades = "https://www.okcoin.cn/api/v1/trades.do";
	url_kline = "https://www.okcoin.cn/api/v1/kline.do?symbol=%s&type=%s&size=%d&since=%s";

	url_userinfo = "https://www.okcoin.cn/api/v1/userinfo.do";
	url_trade = "https://www.okcoin.cn/api/v1/trade.do";
	url_cancel_order = "https://www.okcoin.cn/api/v1/cancel_order.do";
	url_order_info = "https://www.okcoin.cn/api/v1/order_info.do";
	url_orders_info = "https://www.okcoin.cn/api/v1/orders_info.do"
	order_history_uri = "order_history.do";
)

type OKCoinCN_API struct{
	client *http.Client
	api_key string
	secret_key string
}

func currencyPair2String(currency CurrencyPair) string{
	switch currency{
		case BTC_CNY:
			return "btc_cny";
		case LTC_CNY:
			return "ltc_cny";
		default:
			return "";
	}
}

func New(client *http.Client, api_key, secret_key string) * OKCoinCN_API{
	return &OKCoinCN_API{client, api_key, secret_key};
}

func (ctx *OKCoinCN_API) buildPostForm(postForm *url.Values) error {
	postForm.Set("api_key", ctx.api_key);
	//postForm.Set("secret_key", ctx.secret_key);

	payload := postForm.Encode();
	payload = payload + "&secret_key=" +ctx.secret_key;

	sign, err := GetParamMD5Sign(ctx.secret_key, payload);
	if err != nil {
		return err;
	}

	postForm.Set("sign", strings.ToUpper(sign));
	//postForm.Del("secret_key")
	return nil;
}

func (ctx *OKCoinCN_API) placeOrder(side , amount , price string , currency CurrencyPair)(*Order ,error)  {
	postData := url.Values{};
	postData.Set("type" , side);
	postData.Set("amount" , amount);
	postData.Set("price" , price);
	postData.Set("symbol" , currencyPair2String(currency));

	err := ctx.buildPostForm(&postData);

	if err != nil{
		return nil, err;
	}

	body, err := HttpPostForm(ctx.client , url_trade , postData);

	if err != nil{
		return nil, err;
	}

	//println(string(body));

	var respMap map[string]interface{};

	err = json.Unmarshal(body , &respMap);

	if err != nil {
		return nil , err;
	}

	if !respMap["result"].(bool){
		return nil , errors.New(string(body));
	}

	order := new(Order);
	order.OrderID = int(respMap["order_id"].(float64));
	order.Price, _ = strconv.ParseFloat(price, 64);
	order.Amount, _ = strconv.ParseFloat(amount, 64);
	order.Currency = currency;
	order.Status = ORDER_UNFINISH;

	switch side {
	case "buy":
		order.Side = BUY;
	case "sell":
		order.Side = SELL;
	}

	return  order , nil;

}

func (ctx * OKCoinCN_API) LimitBuy(amount, price string, currency CurrencyPair) (*Order, error){
	return ctx.placeOrder("buy" , amount ,price,currency);
}


func (ctx * OKCoinCN_API) LimitSell(amount, price string, currency CurrencyPair) (*Order, error) {
	return ctx.placeOrder("sell" , amount ,price  ,currency);
}

func (ctx * OKCoinCN_API) CancelOrder(orderId string, currency CurrencyPair) (bool, error){
	postData := url.Values{};
	postData.Set("order_id" , orderId);
	postData.Set("symbol" , currencyPair2String(currency));

	ctx.buildPostForm(&postData);

	body , err := HttpPostForm(ctx.client , url_cancel_order , postData);

	if err != nil{
		return false, err;
	}

	var respMap map[string]interface{};

	err = json.Unmarshal(body , &respMap);

	if err != nil {
		return false , err;
	}

	if !respMap["result"].(bool) {
		return false , errors.New(string(body));
	}

	return true, nil;
}

func (ctx *OKCoinCN_API) getOrders(orderId string , currency CurrencyPair)([]Order , error){
	postData := url.Values{};
	postData.Set("order_id" , orderId);
	postData.Set("symbol" , currencyPair2String(currency));

	ctx.buildPostForm(&postData);

	body, err := HttpPostForm(ctx.client , url_order_info , postData);

	if err != nil{
		return nil, err;
	}

	var respMap map[string]interface{};

	err = json.Unmarshal(body , &respMap);

	if err != nil {
		return nil , err;
	}

	if !respMap["result"].(bool) {
		return nil , errors.New(string(body));
	}

	orders := respMap["orders"].([]interface{});

	var orderAr []Order;

	for _ , v := range orders  {
		orderMap := v.(map[string]interface{});

		var order Order;
		order.Currency = currency;
		order.OrderID = int(orderMap["order_id"].(float64));
		order.Amount = orderMap["amount"].(float64);
		order.Price = orderMap["price"].(float64);
		order.DealAmount = orderMap["deal_amount"].(float64);
		order.AvgPrice = orderMap["avg_price"].(float64);
		order.OrderTime = int(orderMap["create_date"].(float64));

		//status:-1:已撤销  0:未成交  1:部分成交  2:完全成交 4:撤单处理中
		switch int(orderMap["status"].(float64)) {
		case -1:
			order.Status = ORDER_CANCEL;
		case 0:
			order.Status = ORDER_UNFINISH;
		case 1:
			order.Status = ORDER_PART_FINISH;
		case 2:
			order.Status = ORDER_FINISH;
		case 4:
			order.Status = ORDER_CANCEL_ING;
		}

		switch orderMap["type"].(string) {
		case "buy":
			order.Side = BUY;
		case "sell":
			order.Side = SELL;
		}

		orderAr = append(orderAr , order);
	}

	//fmt.Println(orders);

	return orderAr, nil;
}

func (ctx * OKCoinCN_API) GetOneOrder(orderId string, currency CurrencyPair) (*Order, error) {
	orderAr , err := ctx.getOrders(orderId , currency);
	if err != nil {
		return nil , err;
	}

	if len(orderAr) == 0 {
		return nil , nil;
	}

	return &orderAr[0] , nil;
}

func (ctx * OKCoinCN_API) GetUnfinishOrders(currency CurrencyPair) ([]Order, error){
	return ctx.getOrders("-1" ,currency);
}

func (ctx * OKCoinCN_API) GetAccount() (*Account, error){
	postData := url.Values{};
	err := ctx.buildPostForm(&postData);

	if err != nil{
		return nil, err;
	}

	body, err := HttpPostForm(ctx.client , url_userinfo , postData);

	if err != nil{
		return nil, err;
	}

	var respMap map[string]interface{};

	err = json.Unmarshal(body , &respMap);

	if err != nil {
		return nil , err;
	}

	if !respMap["result"].(bool){
		errcode := strconv.FormatFloat(respMap["error_code"].(float64), 'f', 0, 64);
		return nil, errors.New(errcode);
	}

	info := respMap["info"].(map[string]interface{});
	funds := info["funds"].(map[string]interface{});
	asset := funds["asset"].(map[string]interface{});
	free := funds["free"].(map[string]interface{});
	freezed := funds["freezed"].(map[string]interface{});
	
	account := new(Account);
	account.Exchange = ctx.GetExchangeName();
	account.Asset, _ = strconv.ParseFloat(asset["total"].(string), 64);
	account.NetAsset, _ = strconv.ParseFloat(asset["net"].(string), 64);

	var btcSubAccount SubAccount;
	var ltcSubAccount SubAccount;
	var cnySubAccount SubAccount;

	btcSubAccount.Currency = BTC;
	btcSubAccount.Amount, _ = strconv.ParseFloat(free["btc"].(string), 64);
	btcSubAccount.LoanAmount = 0;
	btcSubAccount.ForzenAmount, _ = strconv.ParseFloat(freezed["btc"].(string), 64);

	ltcSubAccount.Currency = LTC;
	ltcSubAccount.Amount, _ = strconv.ParseFloat(free["ltc"].(string), 64);
	ltcSubAccount.LoanAmount = 0;
	ltcSubAccount.ForzenAmount, _ = strconv.ParseFloat(freezed["ltc"].(string), 64);

	cnySubAccount.Currency = CNY;
	cnySubAccount.Amount, _ = strconv.ParseFloat(free["cny"].(string), 64);
	cnySubAccount.LoanAmount = 0;
	cnySubAccount.ForzenAmount, _ = strconv.ParseFloat(freezed["cny"].(string), 64);

	account.SubAccounts = make(map[Currency]SubAccount, 3);
	account.SubAccounts[BTC] = btcSubAccount;
	account.SubAccounts[LTC] = ltcSubAccount;
	account.SubAccounts[CNY] = cnySubAccount;

	return account, nil;
}

func (ctx * OKCoinCN_API) GetTicker(currency CurrencyPair) (*Ticker, error){
	var tickerMap map[string]interface{};
	var ticker Ticker;
	
	url := url_ticker + "?symbol=" + currencyPair2String(currency);
	bodyDataMap, err := HttpGet(url);
	if err != nil{
		return nil, err;
	}

	tickerMap = bodyDataMap["ticker"].(map[string]interface{});
	ticker.Date, _ = strconv.ParseUint(bodyDataMap["date"].(string), 10, 64);
	ticker.Last, _ = strconv.ParseFloat(tickerMap["last"].(string), 64);
	ticker.Buy, _ = strconv.ParseFloat(tickerMap["buy"].(string), 64);
	ticker.Sell, _ = strconv.ParseFloat(tickerMap["sell"].(string), 64);
	ticker.Low, _ = strconv.ParseFloat(tickerMap["low"].(string), 64);
	ticker.High, _ = strconv.ParseFloat(tickerMap["high"].(string), 64);
	ticker.Vol, _ = strconv.ParseFloat(tickerMap["vol"].(string), 64);

	return &ticker, nil;
}

func (ctx * OKCoinCN_API) GetDepth(size int, currency CurrencyPair) (*Depth, error){
	var depth Depth;
	
	url := url_depth + "?symbol=" + currencyPair2String(currency) + "&size=" + strconv.Itoa(size);
	bodyDataMap, err := HttpGet(url);
	if err != nil {
		return nil, err;
	}

	for _, v := range bodyDataMap["asks"].([]interface{}) {
		var dr DepthRecord;
		for i, vv := range v.([]interface{}) {
			switch i {
			case 0:
				dr.Price = vv.(float64);
			case 1:
				dr.Amount = vv.(float64);
			}
		}
		depth.AskList = append(depth.AskList, dr);
	}

	for _, v := range bodyDataMap["bids"].([]interface{}) {
		var dr DepthRecord;
		for i, vv := range v.([]interface{}) {
			switch i {
			case 0:
				dr.Price = vv.(float64);
			case 1:
				dr.Amount = vv.(float64);
			}
		}
		depth.BidList = append(depth.BidList, dr);
	}

	return &depth, nil;
}

func (ctx * OKCoinCN_API) GetExchangeName() string{
	return EXCHANGE_NAME;
}

func (ctx *OKCoinCN_API)GetKlineRecords(currency CurrencyPair ,period string, size, since int) ([]Kline , error){

	klineUrl := fmt.Sprintf(url_kline , currencyPair2String(currency) , period , size , since);

	resp , err := http.Get(klineUrl);

	if err != nil {
		return nil , err;
	}

	defer resp.Body.Close();

	body , err := ioutil.ReadAll(resp.Body);

	var klines [][]interface{};

	err = json.Unmarshal(body , &klines);

	if err != nil {
		return nil , err;
	}

	var klineRecords []Kline;

	for _ , record := range klines  {
		r := Kline{};
		for i , e := range record  {
			switch i {
			case 0:
				r.Timestamp = int64(e.(float64))/1000 ;//to unix timestramp
			case 1:
				r.Open = e.(float64);
			case 2:
				r.High = e.(float64);
			case 3:
				r.Low = e.(float64);
			case 4:
				r.Close = e.(float64);
			case 5:
				r.Vol = e.(float64);
			}
		}
		klineRecords = append(klineRecords , r);
	}

	return klineRecords , nil;
}

func (ctx *OKCoinCN_API) GetOrderHistorys(currency CurrencyPair , currentPage , pageSize int)([]Order , error){
	orderHistoryUrl := api_base_url + order_history_uri;

	postData := url.Values{};
	postData.Set("status" , "1");
	postData.Set("symbol" , CurrencyPairSymbol[currency]);
	postData.Set("current_page" , fmt.Sprintf("%d" , currentPage));
	postData.Set("page_length" , fmt.Sprintf("%d" , pageSize));

	err := ctx.buildPostForm(&postData);

	if err != nil{
		return nil, err;
	}

	body, err := HttpPostForm(ctx.client , orderHistoryUrl , postData);

	if err != nil{
		return nil, err;
	}

	var respMap map[string]interface{};

	err = json.Unmarshal(body , &respMap);

	if err != nil {
		return nil , err;
	}

	if !respMap["result"].(bool) {
		return nil , errors.New(string(body));
	}

	orders := respMap["orders"].([]interface{});

	var orderAr []Order;

	for _ , v := range orders  {
		orderMap := v.(map[string]interface{});

		var order Order;
		order.Currency = currency;
		order.OrderID = int(orderMap["order_id"].(float64));
		order.Amount = orderMap["amount"].(float64);
		order.Price = orderMap["price"].(float64);
		order.DealAmount = orderMap["deal_amount"].(float64);
		order.AvgPrice = orderMap["avg_price"].(float64);
		order.OrderTime = int(orderMap["create_date"].(float64));

		//status:-1:已撤销  0:未成交  1:部分成交  2:完全成交 4:撤单处理中
		switch int(orderMap["status"].(float64)) {
		case -1:
			order.Status = ORDER_CANCEL;
		case 0:
			order.Status = ORDER_UNFINISH;
		case 1:
			order.Status = ORDER_PART_FINISH;
		case 2:
			order.Status = ORDER_FINISH;
		case 4:
			order.Status = ORDER_CANCEL_ING;
		}

		switch orderMap["type"].(string) {
		case "buy":
			order.Side = BUY;
		case "sell":
			order.Side = SELL;
		}

		orderAr = append(orderAr , order);
	}

	return orderAr , nil;
}