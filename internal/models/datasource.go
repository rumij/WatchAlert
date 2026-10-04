package models

type AlertDataSource struct {
	TenantId         string                 `json:"tenantId"`
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Labels           map[string]interface{} `json:"labels" gorm:"labels;serializer:json"` // 额外标签，会添加到事件Metric中，可用于区分数据来源；
	Type             string                 `json:"type"`
	HTTP             HTTP                   `json:"http" gorm:"http;serializer:json"`
	Write            Write                  `json:"write" gorm:"write;serializer:json"`
	Auth             Auth                   `json:"Auth" gorm:"auth;serializer:json"`
	DsAliCloudConfig DsAliCloudConfig       `json:"dsAliCloudConfig" gorm:"dsAliCloudConfig;serializer:json"`
	AWSCloudWatch    AWSCloudWatch          `json:"awsCloudwatch" gorm:"awsCloudwatch;serializer:json"`
	ClickHouseConfig DsClickHouseConfig     `json:"clickhouseConfig" gorm:"clickhouseConfig;serializer:json"`
	ZabbixConfig     DsZabbixConfig         `json:"zabbixConfig" gorm:"zabbixConfig;serializer:json"`
	Description      string                 `json:"description"`
	KubeConfig       string                 `json:"kubeConfig"`
	UpdateBy         string                 `json:"updateBy"`
	UpdateAt         int64                  `json:"updateAt"`
	Enabled          *bool                  `json:"enabled" `
}

type Write struct {
	Enabled string `json:"enabled" `
	URL     string `json:"url"`
}

type HTTP struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Timeout int64             `json:"timeout"`
}

type Auth struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type DsClickHouseConfig struct {
	Addr    string
	Timeout int64
}

type DsAliCloudConfig struct {
	AliCloudEndpoint string `json:"alicloudEndpoint"`
	AliCloudAk       string `json:"alicloudAk"`
	AliCloudSk       string `json:"alicloudSk"`
}

// DsZabbixConfig defines polling against the Zabbix JSON-RPC API.
type DsZabbixConfig struct {
	APIToken           string            `json:"apiToken"`
	FaultCenterId      string            `json:"faultCenterId"`
	PollInterval       int64             `json:"pollInterval"`    // seconds, minimum 5, default 15
	Timeout            int64             `json:"timeout"`         // seconds, default 10
	InitialLookback    int64             `json:"initialLookback"` // seconds, default 3600
	Overlap            int64             `json:"overlap"`         // seconds, default 300
	InsecureSkipVerify bool              `json:"insecureSkipVerify"`
	SeverityMapping    map[string]string `json:"severityMapping"`
}

func (c DsZabbixConfig) GetPollInterval() int64 {
	if c.PollInterval < 5 {
		return 15
	}
	return c.PollInterval
}

func (c DsZabbixConfig) GetTimeout() int64 {
	if c.Timeout <= 0 {
		return 10
	}
	return c.Timeout
}

func (c DsZabbixConfig) GetInitialLookback() int64 {
	if c.InitialLookback <= 0 {
		return 3600
	}
	return c.InitialLookback
}

func (c DsZabbixConfig) GetOverlap() int64 {
	if c.Overlap < 0 {
		return 0
	}
	if c.Overlap == 0 {
		return 300
	}
	return c.Overlap
}

type ZabbixSyncState struct {
	DatasourceId    string `json:"datasourceId" gorm:"primaryKey"`
	TenantId        string `json:"tenantId" gorm:"index"`
	LastEventSyncAt int64  `json:"lastEventSyncAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}

func (ZabbixSyncState) TableName() string { return "w8t_zabbix_sync_state" }

type AWSCloudWatch struct {
	//Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

//type PromQueryRes struct {
//	Data data `json:"data"`
//}
//
//type data struct {
//	Result     []result `json:"result"`
//	ResultType string   `json:"resultType"`
//}
//
//type result struct {
//	Metric map[string]interface{} `json:"metric"`
//	Value  []interface{}          `json:"value"`
//}

func (d *AlertDataSource) GetEnabled() bool {
	if d.Enabled == nil {
		isOk := false
		return isOk
	}
	return *d.Enabled
}
