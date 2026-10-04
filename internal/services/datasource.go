package services

import (
	"fmt"
	"time"
	"watchAlert/alert"
	"watchAlert/internal/ctx"
	"watchAlert/internal/models"
	"watchAlert/internal/types"
	"watchAlert/pkg/client"
	"watchAlert/pkg/provider"
	"watchAlert/pkg/tools"

	"github.com/zeromicro/go-zero/core/logc"
)

const zabbixTokenMask = "******"

type datasourceService struct {
	ctx *ctx.Context
}

type InterDatasourceService interface {
	Create(req interface{}) (interface{}, interface{})
	Update(req interface{}) (interface{}, interface{})
	Delete(req interface{}) (interface{}, interface{})
	List(req interface{}) (interface{}, interface{})
	Get(req interface{}) (interface{}, interface{})
	WithAddClientToProviderPools(datasource models.AlertDataSource) error
	WithRemoveClientForProviderPools(datasourceId string)
	ZabbixSyncStatus(req interface{}) (interface{}, interface{})
}

func newInterDatasourceService(ctx *ctx.Context) InterDatasourceService {
	return &datasourceService{
		ctx: ctx,
	}
}

func (ds datasourceService) Create(req interface{}) (interface{}, interface{}) {
	dataSource := req.(*types.RequestDatasourceCreate)

	data := models.AlertDataSource{
		TenantId:         dataSource.TenantId,
		ID:               "ds-" + tools.RandId(),
		Name:             dataSource.Name,
		Labels:           dataSource.Labels,
		Type:             dataSource.Type,
		HTTP:             dataSource.HTTP,
		Write:            dataSource.Write,
		Auth:             dataSource.Auth,
		DsAliCloudConfig: dataSource.DsAliCloudConfig,
		AWSCloudWatch:    dataSource.AWSCloudWatch,
		ClickHouseConfig: dataSource.ClickHouseConfig,
		ZabbixConfig:     dataSource.ZabbixConfig,
		Description:      dataSource.Description,
		KubeConfig:       dataSource.KubeConfig,
		UpdateBy:         dataSource.UpdateBy,
		UpdateAt:         time.Now().Unix(),
		Enabled:          dataSource.Enabled,
	}
	if err := ds.validateDatasource(data); err != nil {
		return nil, err
	}

	err := ds.ctx.DB.Datasource().Create(data)
	if err != nil {
		return nil, err
	}

	if data.GetEnabled() {
		err = ds.WithAddClientToProviderPools(data)
		if err != nil {
			return nil, err
		}
	}
	ds.reloadZabbix(data, tools.ActionCreate)

	return nil, nil
}

func (ds datasourceService) Update(req interface{}) (interface{}, interface{}) {
	dataSource := req.(*types.RequestDatasourceUpdate)
	oldData, err := ds.ctx.DB.Datasource().Get(dataSource.ID)
	if err != nil {
		return nil, err
	}
	if dataSource.Type == provider.ZabbixDsProviderName && (dataSource.ZabbixConfig.APIToken == "" || dataSource.ZabbixConfig.APIToken == zabbixTokenMask) {
		dataSource.ZabbixConfig.APIToken = oldData.ZabbixConfig.APIToken
	}

	data := models.AlertDataSource{
		TenantId:         dataSource.TenantId,
		ID:               dataSource.ID,
		Name:             dataSource.Name,
		Labels:           dataSource.Labels,
		Type:             dataSource.Type,
		HTTP:             dataSource.HTTP,
		Write:            dataSource.Write,
		Auth:             dataSource.Auth,
		DsAliCloudConfig: dataSource.DsAliCloudConfig,
		AWSCloudWatch:    dataSource.AWSCloudWatch,
		ClickHouseConfig: dataSource.ClickHouseConfig,
		ZabbixConfig:     dataSource.ZabbixConfig,
		Description:      dataSource.Description,
		KubeConfig:       dataSource.KubeConfig,
		UpdateBy:         dataSource.UpdateBy,
		UpdateAt:         time.Now().Unix(),
		Enabled:          dataSource.Enabled,
	}
	if err := ds.validateDatasource(data); err != nil {
		return nil, err
	}

	err = ds.ctx.DB.Datasource().Update(data)
	if err != nil {
		return nil, err
	}

	if data.GetEnabled() {
		err = ds.WithAddClientToProviderPools(data)
		if err != nil {
			return nil, err
		}
	} else {
		ds.WithRemoveClientForProviderPools(data.ID)
	}
	if oldData.Type == provider.ZabbixDsProviderName && data.Type != provider.ZabbixDsProviderName {
		ds.reloadZabbix(oldData, tools.ActionDelete)
	}
	ds.reloadZabbix(data, tools.ActionUpdate)

	return nil, nil
}

func (ds datasourceService) Delete(req interface{}) (interface{}, interface{}) {
	dataSource := req.(*types.RequestDatasourceQuery)
	oldData, getErr := ds.ctx.DB.Datasource().Get(dataSource.ID)
	if getErr != nil {
		return nil, getErr
	}
	err := ds.ctx.DB.Datasource().Delete(dataSource.TenantId, dataSource.ID)
	if err != nil {
		return nil, err
	}

	ds.WithRemoveClientForProviderPools(dataSource.ID)
	ds.reloadZabbix(oldData, tools.ActionDelete)

	return nil, nil
}

func (ds datasourceService) Get(req interface{}) (interface{}, interface{}) {
	dataSource := req.(*types.RequestDatasourceQuery)
	data, err := ds.ctx.DB.Datasource().Get(dataSource.ID)
	if err != nil {
		return nil, err
	}

	return maskZabbixToken(data), nil
}

func (ds datasourceService) List(req interface{}) (interface{}, interface{}) {
	var newData []models.AlertDataSource
	dataSource := req.(*types.RequestDatasourceQuery)
	data, err := ds.ctx.DB.Datasource().List(dataSource.TenantId, dataSource.ID, dataSource.Type, dataSource.Query)
	if err != nil {
		return nil, err
	}
	for _, item := range data {
		newData = append(newData, maskZabbixToken(item))
	}

	return newData, nil
}

func (ds datasourceService) WithAddClientToProviderPools(datasource models.AlertDataSource) error {
	var (
		cli interface{}
		err error
	)
	pools := ds.ctx.Redis.ProviderPools()
	switch datasource.Type {
	case provider.PrometheusDsProvider:
		cli, err = provider.NewPrometheusClient(datasource)
	case provider.LokiDsProviderName:
		cli, err = provider.NewLokiClient(datasource)
	case provider.AliCloudSLSDsProviderName:
		cli, err = provider.NewAliCloudSlsClient(datasource)
	case provider.ElasticSearchDsProviderName:
		cli, err = provider.NewElasticSearchClient(ctx.Ctx, datasource)
	case provider.VictoriaLogsDsProviderName:
		cli, err = provider.NewVictoriaLogsClient(ctx.Ctx, datasource)
	case provider.JaegerDsProviderName:
		cli, err = provider.NewJaegerClient(datasource)
	case "Kubernetes":
		cli, err = provider.NewKubernetesClient(ds.ctx.Ctx, datasource.KubeConfig, datasource.Labels)
	case "CloudWatch":
		cli, err = provider.NewAWSCredentialCfg(datasource.AWSCloudWatch.Region, datasource.AWSCloudWatch.AccessKey, datasource.AWSCloudWatch.SecretKey, datasource.Labels)
	case "ClickHouse":
		cli, err = provider.NewClickHouseClient(ctx.Ctx, datasource)
	case provider.ZabbixDsProviderName:
		cli, err = provider.NewZabbixClient(datasource)
	default:
		return fmt.Errorf("unsupported datasource type: %s", datasource.Type)
	}

	if err != nil {
		return fmt.Errorf("New %s client failed, err: %s", datasource.Type, err.Error())
	}

	pools.SetClient(datasource.ID, cli)
	return nil
}

func (ds datasourceService) validateDatasource(data models.AlertDataSource) error {
	if data.Type != provider.ZabbixDsProviderName {
		return nil
	}
	if data.ZabbixConfig.FaultCenterId == "" {
		return fmt.Errorf("Zabbix faultCenterId is required")
	}
	if _, err := provider.NewZabbixClient(data); err != nil {
		return err
	}
	if _, err := ds.ctx.DB.FaultCenter().Get(data.TenantId, data.ZabbixConfig.FaultCenterId, ""); err != nil {
		return fmt.Errorf("Zabbix fault center %s not found: %w", data.ZabbixConfig.FaultCenterId, err)
	}
	return nil
}

func (ds datasourceService) reloadZabbix(data models.AlertDataSource, action string) {
	if data.Type != provider.ZabbixDsProviderName || alert.ZabbixSync == nil {
		return
	}
	if !data.GetEnabled() && action != tools.ActionDelete {
		action = tools.ActionDisable
	}
	if alert.IsLeader() {
		if action == tools.ActionDelete || action == tools.ActionDisable {
			alert.ZabbixSync.Stop(data.ID)
			return
		}
		if err := alert.ZabbixSync.Start(data); err != nil {
			logc.Errorf(ds.ctx.Ctx, "启动 Zabbix 同步失败, datasource=%s(%s), err=%v", data.Name, data.ID, err)
		}
		return
	}
	if err := tools.PublishReloadMessage(ds.ctx.Ctx, client.Redis, tools.ChannelZabbixReload, tools.ReloadMessage{
		Action: action, ID: data.ID, TenantID: data.TenantId, Name: data.Name,
	}); err != nil {
		logc.Errorf(ds.ctx.Ctx, "发布 Zabbix 数据源重载消息失败: %v", err)
	}
}

func maskZabbixToken(data models.AlertDataSource) models.AlertDataSource {
	if data.Type == provider.ZabbixDsProviderName && data.ZabbixConfig.APIToken != "" {
		data.ZabbixConfig.APIToken = zabbixTokenMask
	}
	return data
}

func (ds datasourceService) WithRemoveClientForProviderPools(datasourceId string) {
	pools := ds.ctx.Redis.ProviderPools()
	pools.RemoveClient(datasourceId)
}

func (ds datasourceService) ZabbixSyncStatus(req interface{}) (interface{}, interface{}) {
	request := req.(*types.RequestDatasourceQuery)
	datasource, err := ds.ctx.DB.Datasource().Get(request.ID)
	if err != nil {
		return nil, err
	}
	if datasource.TenantId != request.TenantId {
		return nil, fmt.Errorf("datasource not found")
	}
	if datasource.Type != provider.ZabbixDsProviderName {
		return nil, fmt.Errorf("datasource %s is not a Zabbix datasource", request.ID)
	}
	if alert.ZabbixSync == nil {
		return nil, fmt.Errorf("Zabbix synchronizer is not initialized")
	}
	return alert.ZabbixSync.Status(request.ID), nil
}
