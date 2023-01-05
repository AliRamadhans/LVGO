package main
//LeeVersion_BotLine
//IDLINE : inikah_rasanya
//IDLINE : lee_salsa04
//GUNAKAN OTAK KANAN MU
//Since 2022
import (
	"fmt"
	"github.com/bot-sakura/frugal"
	"github.com/google/uuid"
	"github.com/line-api/line/pkg/logger"
	"github.com/line-api/model/go/model"
	"github.com/phuslu/log"
	"golang.org/x/xerrors"
	"github.com/line-api/line"
	"github.com/line-api/model/go/model"
	"github.com/line-api/line/crypt"
	"time"
	"os"
	"strings"
	"math/rand"
	"encoding/json"
	"io/ioutil"
)
//===================================
//  ISI VARIABLE
//=====================================
// ClientSetting line client setting
type ClientSetting struct {
	AppType        model.ApplicationType
	Proxy          string
	LocalAddr      string
	KeeperDir      string
	AfterTalkError map[model.TalkErrorCode]func(err *model.TalkException) error `json:"-"`

	Logger *log.Logger `json:"-"`
}

type ClientInfo struct {
	Device      *model.Device
	PhoneNumber *model.UserPhoneNumber
}

// Client line client
type Client struct {
	*PollService
	*ChannelService
	*TalkService
	*AccessTokenRefreshService
	*E2EEService
	*NewRegistrationService

	HeaderFactory *HeaderFactory

	opts            []ClientOption
	ctx             frugal.FContext
	ClientSetting   *ClientSetting
	ClientInfo      *ClientInfo
	RequestSequence int32
	ThriftFactory   *ThriftFactory `json:"-"`

	Profile  *model.Profile
	Settings *model.Settings

	TokenManager *TokenManager

	E2EEKeyStore *E2EEKeyStore
}

func (cl *Client) setupSessions() error {
	cl.PollService = cl.newPollService()
	cl.ChannelService = cl.newChannelService()
	cl.TalkService = cl.newTalkService()
	cl.AccessTokenRefreshService = cl.newAccessTokenRefreshService()
	cl.E2EEService = cl.newE2EEService()
	cl.NewRegistrationService = cl.newNewRegistrationService()
	return nil
}

func (cl *Client) executeOpts() error {
	for idx, opt := range cl.opts {
		err := opt(cl)
		if err != nil {
			return xerrors.Errorf("failed to execute %v option: %w", idx, err)
		}
	}
	return nil
}

func (cl *Client) GetLineApplicationHeader() string {
	switch cl.ClientSetting.AppType {
	case model.ApplicationType_ANDROID:
		return fmt.Sprintf("ANDROID\t%v\tAndroid OS\t%v", cl.HeaderFactory.AndroidAppVersion, cl.HeaderFactory.AndroidVersion)
	case model.ApplicationType_ANDROIDSECONDARY:
		return fmt.Sprintf("ANDROIDSECONDARY\t%v\tAndroid OS\t%v", cl.HeaderFactory.AndroidSecondaryAppVersion, cl.HeaderFactory.AndroidVersion)
	case model.ApplicationType_IOS:
		return "IOS\t12.6.0\tiOS\t16.0"
	}
	panic("unsupported app type")
}

func (cl *Client) GetLineUserAgentHeader() string {
	switch cl.ClientSetting.AppType {
	case model.ApplicationType_ANDROID:
		return fmt.Sprintf("Line/%v", cl.HeaderFactory.AndroidAppVersion)
	case model.ApplicationType_ANDROIDSECONDARY:
		return fmt.Sprintf("Line/%v %v %v", cl.HeaderFactory.AndroidSecondaryAppVersion, cl.ClientInfo.Device.DeviceModel, cl.HeaderFactory.AndroidVersion)
	case model.ApplicationType_IOS:
		return "Line/12.6.0"
	}
	panic("unsupported app type")
}

func newLineDevice() *model.Device {
	uuidObj, _ := uuid.NewUUID()
	return &model.Device{
		Udid:        strings.Join(strings.Split(uuidObj.String(), "-"), ""),
		DeviceModel: genRandomDeviceModel(),
	}
}

func getHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// create default line client
func newDefaultClient() *Client {
	cl := &Client{
		ctx: frugal.NewFContext(""),
		ClientSetting: &ClientSetting{
			AppType:   model.ApplicationType_ANDROID,
			KeeperDir: getHomeDir() + "/.line-keepers/",
			Logger:    logger.New(),
		},
		ClientInfo: &ClientInfo{
			Device: newLineDevice(),
		},
		TokenManager: &TokenManager{},
		Profile:      &model.Profile{},
		Settings:     &model.Settings{},
		HeaderFactory: &HeaderFactory{
			AndroidVersion:        getRandomAndroidVersion(),
			AndroidAppVersion:     getRandomAndroidAppVersion(),
			AndroidLiteAppVersion: getRandomAndroidSecondaryAppVersion(),
		},
		E2EEKeyStore: NewE2EEKeyStore(),
	}
	cl.ClientSetting.AfterTalkError = map[model.TalkErrorCode]func(err *model.TalkException) error{
		model.TalkErrorCode_MUST_REFRESH_V3_TOKEN: func(talkErr *model.TalkException) error {
			err := cl.RefreshV3AccessToken()
			if err != nil {
				return err
			}
			return xerrors.Errorf("update v3 access token done: %w", talkErr)
		},
	}
	return cl
}

// New create new line client
func New(opts ...ClientOption) *Client {
	cl := newDefaultClient()
	cl.opts = opts
	return cl
}
//=========== batas client.go =================

type ClientOption func(client *Client) error

// ApplicationType set line client application type
func ApplicationType(appType model.ApplicationType) ClientOption {
	return func(client *Client) error {
		client.ClientSetting.AppType = appType
		return nil
	}
}

// Proxy set line client proxy
func Proxy(proxy string) ClientOption {
	return func(client *Client) error {
		client.ClientSetting.Proxy = proxy
		return nil
	}
}

// KeeperDir set line client keepers path
func KeeperDir(path string) ClientOption {
	return func(client *Client) error {
		client.ClientSetting.KeeperDir = path
		return nil
	}
}

func LocalAddr(addr string) ClientOption {
	return func(client *Client) error {
		client.ClientSetting.LocalAddr = addr
		return nil
	}
}

func AfterTalkError(fncs map[model.TalkErrorCode]func(err *model.TalkException) error) ClientOption {
	return func(client *Client) error {
		for k, v := range fncs {
			client.ClientSetting.AfterTalkError[k] = v
		}
		return nil
	}
}

func Logger(logger *log.Logger) ClientOption {
	return func(client *Client) error {
		client.ClientSetting.Logger = logger
		return nil
	}
}
//========== batas client_option.go ================

type Path string

const (
	PATH_LONG_POLLING                             Path = "/P4"
	PATH_LONG_POLLING_P5                          Path = "/P5"
	PATH_NORMAL_POLLING                           Path = "/NP4"
	PATH_NORMAL                                   Path = "/S4"
	PATH_COMPACT_MESSAGE                          Path = "/C5"
	PATH_COMPACT_PLAIN_MESSAGE                    Path = "/CA5"
	PATH_COMPACT_E2EE_MESSAGE                     Path = "/ECA5"
	PATH_REGISTRATION                             Path = "/api/v4/TalkService.do"
	PATH_REFRESH_TOKEN                            Path = "/EXT/auth/tokenrefresh/v1"
	PATH_NOTIFY_SLEEP                             Path = "/F4"
	PATH_NOTIFY_BACKGROUND                        Path = "/B"
	PATH_BUDDY                                    Path = "/BUDDY4"
	PATH_SHOP                                     Path = "/SHOP4"
	PATH_SHOP_AUTH                                Path = "/SHOPA"
	PATH_UNIFIED_SHOP                             Path = "/TSHOP4"
	PATH_STICON                                   Path = "/SC4"
	PATH_CHANNEL                                  Path = "/CH4"
	PATH_CANCEL_LONGPOLLING                       Path = "/CP4"
	PATH_SNS_ADAPTER                              Path = "/SA4"
	PATH_SNS_ADAPTER_REGISTRATION                 Path = "/api/v4p/sa"
	PATH_AUTH_EAP                                 Path = "/ACCT/authfactor/eap/v1"
	PATH_USER_INPUT                               Path = ""
	PATH_USER_BEHAVIOR_LOG                        Path = "/L1"
	PATH_AGE_CHECK                                Path = "/ACS4"
	PATH_SPOT                                     Path = "/SP4"
	PATH_CALL                                     Path = "/V4"
	PATH_EXTERNAL_INTERLOCK                       Path = "/EIS4"
	PATH_TYPING                                   Path = "/TS"
	PATH_CONN_INFO                                Path = "/R2"
	PATH_HTTP_PROXY                               Path = ""
	PATH_EXTERNAL_PROXY                           Path = ""
	PATH_PAY                                      Path = "/PY4"
	PATH_WALLET                                   Path = "/WALLET4"
	PATH_AUTH                                     Path = "/RS4"
	PATH_AUTH_REGISTRATION                        Path = "/api/v4p/rs"
	PATH_SEARCH_COLLECTION_MENU_V1                Path = "/collection/v1"
	PATH_SEARCH_V2                                Path = "/search/v2"
	PATH_SEARCH_V3                                Path = "/search/v3"
	PATH_BEACON                                   Path = "/BEACON4"
	PATH_PERSONA                                  Path = "/PS4"
	PATH_SQUARE                                   Path = "/SQS1"
	PATH_SQUARE_BOT                               Path = "/BP1"
	PATH_POINT                                    Path = "/POINT4"
	PATH_COIN                                     Path = "/COIN4"
	PATH_LIFF                                     Path = "/LIFF1"
	PATH_CHAT_APP                                 Path = "/CAPP1"
	PATH_IOT                                      Path = "/IOT1"
	PATH_USER_PROVIDED_DATA                       Path = "/UPD4"
	PATH_NEW_REGISTRATION                         Path = "/acct/pais/v1"
	PATH_SECONDARY_QR_LOGIN                       Path = "/ACCT/lgn/sq/v1"
	PATH_USER_SETTINGS                            Path = "/US4"
	PATH_LINE_SPOT                                Path = "/ex/spot"
	PATH_LINE_HOME_V2_SERVICES                    Path = "/EXT/home/sapi/v4p/hsl"
	PATH_LINE_HOME_V2_CONTENTS_RECOMMENDATIONPath Path = "/EXT/home/sapi/v4p/flex"
	PATH_BIRTHDAY_GIFT_ASSOCIATION                Path = "/EXT/home/sapi/v4p/bdg"
	PATH_SECONDARY_PWLESS_LOGIN_PERMIT            Path = "/ACCT/lgn/secpwless/v1"
	PATH_SECONDARY_AUTH_FACTOR_PIN_CODE           Path = "/ACCT/authfactor/second/pincode/v1"
	PATH_PWLESS_CREDENTIAL_MANAGEMENT             Path = "/ACCT/authfactor/pwless/manage/v1"
	PATH_PWLESS_PRIMARY_REGISTRATION              Path = "/ACCT/authfactor/pwless/v1"
	PATH_GLN_NOTIFICATION_STATUS                  Path = "/gln/webapi/graphql"
	PATH_BOT_EXTERNAL                             Path = "/BOTE"
	PATH_E2EE_KEY_BACKUP                          Path = "/EKBS4"
)

const (
	LINE_SERVER_HOST     = "https://legy-jp-addr.line.naver.jp"
	LINE_SERVER_HOST_gxx = "https://gxx.line.naver.jp"
)

//ToURL get full url for the path
func (p Path) ToURL() string {
	return LINE_SERVER_HOST + string(p)
}

//======== batas config.go ====================

type E2EEKeyStore struct {
	Data map[string]*crypt.E2EEKeyPair

	Status map[string]*crypt.E2EEStatus
}

func NewE2EEKeyStore() *E2EEKeyStore {
	return &E2EEKeyStore{
		Data:   map[string]*crypt.E2EEKeyPair{},
		Status: map[string]*crypt.E2EEStatus{},
	}
}

func (s *E2EEKeyStore) formatKey(keyId int32, mid string) string {
	return fmt.Sprintf("%s_%d", mid, keyId)
}

func (s *E2EEKeyStore) Get(mid string, keyId int32) (*crypt.E2EEKeyPair, bool) {
	key, ok := s.Data[s.formatKey(keyId, mid)]
	if ok {
		s.Status[mid] = &crypt.E2EEStatus{
			SpecVersion: key.Version,
			KeyId:       keyId,
		}
	}
	return key, ok
}

func (s *E2EEKeyStore) GetByMid(mid string) (*crypt.E2EEKeyPair, bool) {
	status, ok := s.Status[mid]
	if ok {
		return s.Get(mid, status.KeyId)
	}
	return nil, false
}

func (s *E2EEKeyStore) Set(mid string, keyId int32, key *crypt.E2EEKeyPair) {
	s.Data[s.formatKey(keyId, mid)] = key
	s.Status[mid] = &crypt.E2EEStatus{
		SpecVersion: key.Version,
		KeyId:       keyId,
	}
}
//========= batas e2ee_store.go =============

func (cl *Client) afterError(err error) error {
	if err == nil {
		return nil
	}
	talkErr, ok := err.(*model.TalkException)
	if !ok {
		return err
	}
	fnc, ok := cl.ClientSetting.AfterTalkError[talkErr.Code]
	if !ok {
		return err
	}
	return fnc(talkErr)
}
//======= batas error.go ===========

var androidVersions = []string{
	"11.0.0", "10.0.0", "9.0.0", "8.1.0", "8.0.0", "7.1.2", "7.1.1", "7.1.0", "7.0.0",
	"6.0.1", "6.0.0", "5.1.1", "5.1.0", "5.0.2", "5.0.1", "5.0.0",
}

func getRandomAndroidVersion() string {
	rand.Seed(time.Now().Unix())
	return androidVersions[rand.Intn(len(androidVersions))]
}

var androidAppVersions = []string{
	"11.17.1", "11.17.0", "11.16.2", "11.16.0", "11.15.3", "11.15.2", "11.15.0", "11.14.3",
}

func getRandomAndroidAppVersion() string {
	rand.Seed(time.Now().Unix())
	return androidAppVersions[rand.Intn(len(androidAppVersions))]
}

var androidSecondaryAppVersions = []string{
	"2.17.1", "2.17.0", "2.16.0", "2.15.0",
}

func getRandomAndroidSecondaryAppVersion() string {
	rand.Seed(time.Now().Unix())
	return androidSecondaryAppVersions[rand.Intn(len(androidLiteAppVersions))]
}

type HeaderFactory struct {
	AndroidVersion        string
	AndroidAppVersion     string
	androidSecondaryAppVersions string
}
//======== batas header_factory.go =============

func CreateDirIfNotExist(dirName string) error {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		err := os.Mkdir(dirName, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

func isPathExist(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func ReadJsonToStruct(fName string, struct_ interface{}) (interface{}, error) {
	file, err := os.Open(fName)
	if err != nil {
		return nil, err
	}
	parser := json.NewDecoder(file)
	err = parser.Decode(struct_)
	return struct_, err
}

func WriteStructToJson(fName string, struct_ interface{}) error {
	file, err := json.MarshalIndent(struct_, "", "    ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fName, file, 0644)
	return err
}

func (cl *Client) SaveKeeper() error {
	err := CreateDirIfNotExist(cl.ClientSetting.KeeperDir)
	if err != nil {
		return err
	}
	path := fmt.Sprintf(cl.ClientSetting.KeeperDir+"/%v.keeper", cl.Profile.Mid)
	return WriteStructToJson(path, cl)
}

func (cl *Client) LoadKeeper() error {
	path := fmt.Sprintf(cl.ClientSetting.KeeperDir+"/%v.keeper", cl.Profile.Mid)
	if isPathExist(path) {
		_, err := ReadJsonToStruct(path, cl)
		return err
	}
	return xerrors.New("keeper file not found")
}
//========== batas keeper.go =========
func main() {
	cl := line.New(line.Proxy(""))
	//err := cl.LoginViaV3Token(
	//	"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJqdGkiOiI5YmRmMTIwNy0zNGM0LTRhZDAtOWVlNC0wNGY5MWZhZjUyZWEiLCJhdWQiOiJMSU5FIiwiaWF0IjoxNjMzMzE0NzYwLCJleHAiOjE2MzM5MTk1NjAsInNjcCI6IkxJTkVfQ09SRSIsInJ0aWQiOiJlZThhMmMwZC1iNDFkLTQ0NWUtYThmOS0yYjQyYzY5YjEwOWMiLCJyZXhwIjoxNzkwOTk0NzYwLCJ2ZXIiOiIzLjEiLCJhaWQiOiJ1MjI2MDY2ODk5NDM2MDVmZTk5ODg4MWY3ZGE0M2NlMGIiLCJsc2lkIjoiNTU4NmQ2Y2MtNGU1NC00Y2IxLWI1MDktOGI3MWM0ZDEzMTdiIiwiZGlkIjoiNDUwOTdhMmIyNGJiMTFlYzk0N2VhOGExNTkzYzllODgiLCJjdHlwZSI6IkFORFJPSUQiLCJjbW9kZSI6IlBSSU1BUlkiLCJjaWQiOiIwMDAwMDAwMDAwIn0.Mi_a5TdIDfQ7x7S14tge1Y2RS9uOkUhApxnjXkM8UNo",
	//	"eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJqdGkiOiI2NmY3MWYwMS1iZWVlLTRjNzUtODY3OS02ZDc0ZjlhZjY3MmUiLCJhdGkiOiI4ZTIyOGRhYS1iMzQyLTRkNmItYjYyNC02Y2MzYzkxZDBmZTEiLCJhdWQiOiJMSU5FIiwicm90IjoiUk9UQVRFIiwiaWF0IjoxNjMzNTQyMjUyLCJleHAiOjE3OTEyMjIyNTIsInNjcCI6IkxJTkVfQ09SRSIsInZlciI6IjMuMSIsImFpZCI6InUzNDQwZTQxNzRkZDcxNDcwODk2ODRjMWQyZTdmODQyNSIsImxzaWQiOiIyZGFiZmIwNi1kMzk0LTRlYjYtODBmMy05MGI3YzhiZDEyMzYiLCJkaWQiOiJmMmVlODcwMzI2Y2MxMWVjYjgzMmE4YTE1OTNjOWU4OCIsImFwcElkIjoiMDAwMDAwMDAwMCJ9.P3qGN4DaPtBnG2q8xyWjvWN5o7BXAz2SLScb7MngW74",
	//)
	err := cl.LoginViaAuthKey("u07000fb16ec97ac70a3decb5b6cad1f7:hcIyDQFDITd9tDpk7xmk")
	if err != nil {
		fmt.Printf("%#v\n", err)
	}
	for true {
		err := cl.SaveKeeper()
		if err != nil {
			fmt.Printf("%#v\n", err)
		}
		ops, err := cl.FetchLineOperationsTMCP()
		if err != nil {
			fmt.Printf("%#v\n", err)
		}
		for _, op := range ops {
			if op.OpType == model.OpType_RECEIVE_MESSAGE {
				fmt.Printf("%#v\n", op.Message.String())
				switch op.Message.Text {
				case "hi":
					cl.SendMessageCompact(&model.Message{
						To:   op.Message.To,
						From: cl.Profile.Mid,
					})
				}
			}
		}
		time.Sleep(time.Duration(250) * time.Millisecond)
	}
}
