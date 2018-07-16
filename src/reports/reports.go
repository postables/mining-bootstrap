package reports

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/RTradeLtd/mining-bootstrap/src/reports/config"
	"github.com/RTradeLtd/mining-bootstrap/src/reports/types"
	sendgrid "github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

/*
This is used to handle automated mining reports for cryptocurrency mining farms
The idea is to create an easy to use system that can be used by farm operators to create accurate book reports for the tax man
*/

const (
	// USDAPI is the URL We use to query for USD->CAD conversion
	USDAPI = "https://free.currencyconverterapi.com/api/v5/convert?q=USD_CAD&compact=y"
)

var methodList = []string{"24hour_credit", "credit"}

// Manager is a helper struct used for report generation
type Manager struct {
	Config         *config.Config   `json:"config"`
	EthUSD         float64          `json:"eth_usd"` // keeps track of the ETH->USD conversion ratio
	UsdCad         float64          `json:"usd_cad"` // keeps track of the USD -> USD conversion ratio
	SendgridClient *sendgrid.Client `json:"sendgrid_client"`
}

// GenerateReportManagerFromFile is used to generate our helper struct from the config file
func GenerateReportManagerFromFile(path string) (*Manager, error) {
	cfg, err := config.LoadConfigFromFile(path)
	if err != nil {
		return nil, err
	}
	usd, err := ParseUSDCAD()
	if err != nil {
		return nil, err
	}
	eth, err := ParseETHUSD()
	if err != nil {
		return nil, err
	}
	return &Manager{Config: cfg, EthUSD: eth, UsdCad: usd, SendgridClient: sendgrid.NewSendClient(cfg.SendgridAPIKey)}, nil

}

// CreateReportAndSend is used to create and send a mining report
func (m *Manager) CreateReportAndSend(method string) error {
	switch method {
	case "24hour_credit":
		credit, err := m.GetRecentCredits24Hours()
		if err != nil {
			return err
		}
		usdValue := credit.Amount * m.EthUSD
		cadValue := usdValue * m.UsdCad
		resp, err := m.Send24HourEmail(credit.Amount, usdValue, cadValue)
		if err != nil {
			return err
		}
		if resp != 202 {
			return fmt.Errorf("unacceptable return code, expected 200 got %v", resp)
		}
	case "credit":
		return fmt.Errorf("not yet supported")
	default:
		return fmt.Errorf("invalid method must be one of %v", methodList)
	}
	return nil
}

// GetRecentCredits24Hours is use the get the number of "credits" (credits being number of coins) mined in the last 24 hour period.
func (m *Manager) GetRecentCredits24Hours() (*types.RecentCredits, error) {
	s := "getdashboarddata"
	m.FormatURL(s)
	resp, err := http.Get(m.Config.URL)
	if err != nil {
		return nil, err
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var intf map[string]interface{}
	var data types.MiningPoolHubAPIResponse
	err = json.Unmarshal(bodyBytes, &intf)
	if err != nil {
		return nil, err
	}
	marshaled, err := json.Marshal(intf[s])
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(marshaled, &data)
	if err != nil {
		return nil, err
	}
	marshaled, err = json.Marshal(data.Data["recent_credits_24hours"])
	if err != nil {
		return nil, err
	}
	var credits types.RecentCredits
	err = json.Unmarshal(marshaled, &credits)
	if err != nil {
		return nil, err
	}
	return &credits, nil
}

// GetRecentCredits is used to get the total number of credits mined over the last 2 week period, broken down into day intervals
func (m *Manager) GetRecentCredits() (*[]types.RecentCredits, error) {
	s := "getdashboarddata"
	m.FormatURL(s)
	resp, err := http.Get(m.Config.URL)
	if err != nil {
		return nil, err
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var intf map[string]interface{}
	var data types.MiningPoolHubAPIResponse
	err = json.Unmarshal(bodyBytes, &intf)
	if err != nil {
		return nil, err
	}
	marshaled, err := json.Marshal(intf[s])
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(marshaled, &data)
	if err != nil {
		return nil, err
	}
	marshaled, err = json.Marshal(data.Data["recent_credits"])
	if err != nil {
		return nil, err
	}
	var credits []types.RecentCredits
	err = json.Unmarshal(marshaled, &credits)
	if err != nil {
		return nil, err
	}
	return &credits, nil
}

// FormatURL is a helper method used to format a URL with the given config information
func (m *Manager) FormatURL(action string) {
	m.Config.URL = fmt.Sprintf(m.Config.URL, m.Config.Coin, action, m.Config.APIKey)
}

// Send24HourEmail is a function used to send report information for the last 24 hour period
func (m *Manager) Send24HourEmail(ethMined, usdValue, cadValue float64) (int, error) {
	content := fmt.Sprintf("<br>Eth Mined: %v<br>USD Value: %v<br>CAD Value: %v", ethMined, usdValue, cadValue)
	from := mail.NewEmail("stake-sendgrid-api", "sgapi@rtradetechnologies.com")
	subject := "Ethereum Mining Report"
	to := mail.NewEmail("Mining Reports", "reports@rtradetechnologies.com")

	mContent := mail.NewContent("text/html", content)
	mail := mail.NewV3MailInit(from, subject, to, mContent)

	response, err := m.SendgridClient.Send(mail)
	if err != nil {
		return 0, err
	}
	return response.StatusCode, nil
}

// SendTemplateEmail is a function that can be used to send any kind of report email
func (m *Manager) SendTemplateEmail(args map[string]string) (int, error) {
	content := args["content"]
	contentType := args["content_type"]
	fromName := args["from_name"]
	fromEmail := args["from_email"]
	subject := args["subject"]
	toName := args["to_name"]
	toEmail := args["to_email"]

	from := mail.NewEmail(fromName, fromEmail)
	to := mail.NewEmail(toName, toEmail)
	mContent := mail.NewContent(contentType, content)
	mail := mail.NewV3MailInit(from, subject, to, mContent)
	response, err := m.SendgridClient.Send(mail)
	if err != nil {
		return 0, err
	}
	return response.StatusCode, nil
}
