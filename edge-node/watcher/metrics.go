package watcher

const (
	MetricsAlaya             = "ecos.node.alaya"
	MetricsAlayaMeta         = MetricsAlaya + ".meta"
	MetricsAlayaMetaCount    = MetricsAlayaMeta + ".count"
	MetricsAlayaMetaPutTimer = MetricsAlayaMeta + ".put.timer"
	MetricsAlayaMetaGetTimer = MetricsAlayaMeta + ".get.timer"

	MetricsAlayaPipeline      = MetricsAlaya + ".pipeline"
	MetricsAlayaPipelineCount = MetricsAlayaPipeline + ".count"
)

const (
	MetricsGaia              = "ecos.node.gaia"
	metricsGaiaBlock         = MetricsGaia + ".block"
	MetricsGaiaBlockCount    = metricsGaiaBlock + ".count"
	MetricsGaiaBlockPutTimer = metricsGaiaBlock + ".put.timer"
	MetricsGaiaBlockGetTimer = metricsGaiaBlock + ".get.timer"
)

const (
	MetricsGateway             = "ecos.node.gateway"
	MetricsGatewayGetTimer     = MetricsGateway + ".get.timer"
	MetricsGatewayPartGetTimer = MetricsGateway + ".part.get.timer"
	MetricsGatewayPutTimer     = MetricsGateway + ".put.timer"
	MetricsGatewayPartPutTimer = MetricsGateway + ".part.put.timer"
)

var MetricsGatewayLabels = []string{
	MetricsGatewayGetTimer,
	MetricsGatewayPartGetTimer,
	MetricsGatewayPutTimer,
	MetricsGatewayPartPutTimer,
}
