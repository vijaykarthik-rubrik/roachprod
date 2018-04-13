package bodega

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cockroachdb/roachprod/vm"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const (
	bodegaConfPathTemplate  = "${HOME}/.bodega.conf.yml"
	bodegaOrderPathTemplate = "${HOME}/.roachprod/bodega_orders"
	urlScheme               = "https"
	urlHost                 = "bodega.rubrik-lab.com"
	urlPathOrders           = "api/orders/"
	urlPathOrderUpdates     = "api/order_updates/"
	urlPathProfile          = "api/profile/"
)

type bodegaManager struct {
	bodegaOrderPath string
	client          httpClient
}

func newBodegaManager() (*bodegaManager, error) {
	token, err := readBodegaToken()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read Bodega token")
	}
	auth := "Token " + token
	path := os.ExpandEnv(bodegaOrderPathTemplate)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := httpClient{auth, http.Client{Transport: tr}}

	return &bodegaManager{path, client}, nil
}

// readBodegaToken reads Bodega token from bodega.conf.yml
func readBodegaToken() (string, error) {
	bodegaConfPath := os.ExpandEnv(bodegaConfPathTemplate)
	data, err := ioutil.ReadFile(bodegaConfPath)
	if err != nil {
		return "", errors.Wrapf(err, "problem reading file %s", bodegaConfPath)
	}

	type bodegaConfig struct {
		URL   string
		Token string
	}

	var conf bodegaConfig
	if err := yaml.Unmarshal(data, &conf); err != nil {
		return "", errors.Wrapf(err, "unable to unmarshal Bodega config")
	}
	if conf.Token != "" {
		return conf.Token, nil
	}
	return "", fmt.Errorf("no field named 'token'")
}

func (m *bodegaManager) postDataToPath(
	path string,
	data map[string]interface{},
) (map[string]interface{}, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to encode JSON: %v", data)
	}

	u := &url.URL{
		Scheme: urlScheme,
		Host:   urlHost,
		Path:   path,
	}
	return m.client.post(u, dataBytes)
}

// placeOrder places an order using names as machine names and returns orderID
func (m *bodegaManager) placeOrder(names []string, opts providerOpts) (string, error) {
	machine := map[string]interface{}{
		"type": "ubuntu_machine",
		"requirements": map[string]interface{}{
			"disk_size": opts.DiskSize,
			"location":  opts.Location,
			"model":     opts.Model,
		},
	}
	items := make(map[string]interface{})
	for _, name := range names {
		items[name] = machine
	}

	itemsBytes, err := json.Marshal(items)
	if err != nil {
		return "", errors.Wrapf(err, "unable to encode JSON: %v", items)
	}
	itemsString := string(itemsBytes)

	order := map[string]interface{}{"items": itemsString}
	result, err := m.postDataToPath(urlPathOrders, order)
	if err != nil {
		return "", err
	}

	orderID, ok := result["sid"]
	if !ok {
		return "", fmt.Errorf("unable to find orderID in POST response")
	}
	return orderID.(string), nil
}

func (m *bodegaManager) closeOrder(orderID string) error {
	data := map[string]interface{}{
		"order_sid":  orderID,
		"new_status": "CLOSED",
		"comment":    "This order is closed by roachprod.",
	}
	_, err := m.postDataToPath(urlPathOrderUpdates, data)
	return err
}

func (m *bodegaManager) extendOrder(orderID string, timeDelta time.Duration) error {
	comment := fmt.Sprintf(
		"This order is extended by roachprod.\nIt has been extended for %s",
		timeDelta,
	)
	data := map[string]interface{}{
		"order_sid":        orderID,
		"time_limit_delta": convert(timeDelta),
		"comment":          comment,
	}
	_, err := m.postDataToPath(urlPathOrderUpdates, data)
	return err
}

// consumeOrder consumes order information from Bodega
func (m *bodegaManager) consumeOrder(orderID string) (map[string]interface{}, error) {
	u := &url.URL{
		Scheme: urlScheme,
		Host:   urlHost,
		Path:   urlPathOrders + orderID,
	}

	return m.client.get(u)
}

// listOrders lists all live orders
func (m *bodegaManager) listOrders() (map[string]interface{}, error) {
	ownerSid, err := m.ownerSid()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get user's sid")
	}

	v := url.Values{}
	v.Set("owner_sid", ownerSid)
	v.Set("status_live", "true")
	u := &url.URL{
		Scheme:   urlScheme,
		Host:     urlHost,
		Path:     urlPathOrders,
		RawQuery: v.Encode(),
	}

	return m.client.get(u)
}

// ownerSid returns current user's sid
func (m *bodegaManager) ownerSid() (string, error) {
	u := &url.URL{
		Scheme: urlScheme,
		Host:   urlHost,
		Path:   urlPathProfile,
	}

	result, err := m.client.get(u)
	if err != nil {
		return "", err
	}

	sid, ok := result["sid"]
	if !ok {
		return "", fmt.Errorf("unable to find sid in GET response")
	}
	sidString := sid.(string)
	return sidString, nil
}

func (m *bodegaManager) fulfilled(orderID string) bool {
	order, err := m.consumeOrder(orderID)
	if err != nil {
		return false
	}

	if status, ok := order["status"]; ok && status == "FULFILLED" {
		return true
	}
	return false
}

// orderIDOfVMs returns the orderID of given machines. It assumes that all machines
// are from the same order and returns the orderID of the first machine.
func (m *bodegaManager) orderIDOfVMs(vms vm.List) (string, error) {
	if len(vms) == 0 {
		return "", fmt.Errorf("no machine provided")
	}
	orderMap, err := m.orderToMachineMap()
	if err != nil {
		return "", errors.Wrapf(err, "unable to get order to machine map")
	}

	for orderID, machines := range orderMap {
		if inSlice(vms[0].Name, machines) {
			return orderID, nil
		}
	}
	return "", fmt.Errorf("unable to find orderID for machines: %v", vms.Names())
}

// liveOrderIDs returns orderIDs of "Open" and "Fulfilled" orders of current user
func (m *bodegaManager) liveOrderIDs() ([]string, error) {
	orders, err := m.listOrders()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to list orders")
	}
	results, ok := orders["results"]
	if !ok {
		return nil, fmt.Errorf("no key named 'results' in orders")
	}

	var orderIDs []string
	for _, order := range results.([]interface{}) {
		orderMap := order.(map[string]interface{})
		orderID := orderMap["sid"].(string)
		orderIDs = append(orderIDs, orderID)
	}
	return orderIDs, nil
}

// orderToMachineMap reads order info from bodegaOrderPath and returns only live orders
func (m *bodegaManager) orderToMachineMap() (map[string][]string, error) {
	liveOrderIDs, err := m.liveOrderIDs()
	if err != nil {
		return nil, err
	}

	var orderMap map[string][]string
	file, err := os.Open(m.bodegaOrderPath)
	if err != nil {
		if os.IsNotExist(err) {
			return orderMap, nil
		}
		return nil, errors.Wrapf(err, "problem opening file %s", m.bodegaOrderPath)
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&orderMap); err != nil {
		return nil, errors.Wrapf(err, "unable to decode file %s", m.bodegaOrderPath)
	}

	liveOrderMap := make(map[string][]string)
	for orderID, machines := range orderMap {
		if inSlice(orderID, liveOrderIDs) {
			liveOrderMap[orderID] = machines
		}
	}
	return liveOrderMap, nil
}

// saveOrderInfo saves orderMap(map of orderID to VM names) to bodegaOrderPath file
func (m *bodegaManager) saveOrderInfo(orderMap map[string][]string) error {
	json, err := json.MarshalIndent(orderMap, "", "  ")
	if err != nil {
		return errors.Wrapf(err, "unable to marshal JSON")
	}

	return ioutil.WriteFile(m.bodegaOrderPath, json, 0644)
}

// addOrderInfo gets live orders, adds current order, and saves back to bodegaOrderPath
func (m *bodegaManager) addOrderInfo(orderID string, names []string) error {
	orderMap, err := m.orderToMachineMap()
	if err != nil {
		return errors.Wrapf(err, "unable to get order to machine map")
	}
	orderMap[orderID] = names
	if err := m.saveOrderInfo(orderMap); err != nil {
		return errors.Wrapf(err, "unable to save order info to file")
	}
	return nil
}
