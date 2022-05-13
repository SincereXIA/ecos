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
