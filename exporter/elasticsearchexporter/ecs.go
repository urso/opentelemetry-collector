package elasticsearchexporter

// opentelemetry semantic conventions: https://github.com/open-telemetry/opentelemetry-specification/tree/master/semantic_conventions
// ECS schema: https://www.elastic.co/guide/en/ecs/current/index.html
var ecsConventionsMapping = map[string]string{
	// resource/cloud
	// "cloud.provider" -> Keep
	// "cloud.region" -> Keep
	// "cloud.account.id" -> Keep
	"cloud.zone": "cloud.availability_zone",

	// resource/container
	// "container.name" -> Keep
	// "container.id" -> Keep
	// "container.image.name" -> Keep
	// "container.image.tag" -> Keep

	// resource/deployment_environment
	// "deployment.environment" ->  TODO? (merge with 'tags'?)

	// resource/faas
	"faas.name":     "agent.name",
	"faas.id":       "agent.id",
	"faas.version":  "agent.verson",
	"faas.instance": "agent.ephemeral_id",

	// resource/host
	// "host.id" -> Keep
	// "host.name" -> translates to host.name or host.hostname, depending on actual contents...
	// "host.type" -> Keep
	// "host.image.name" -> TODO
	// "host.image.id" -> TODO
	// "host.image.version" -> TODO

	// resource/k8s TODO
	// "k8s.cluster.name"
	// "k8s.namespace.name"
	// "k8s.pod.uid"
	// "k8s.pod.name"
	// "k8s.container.name"
	// "k8s.replicaset.uid"
	// "k8s.replicaset.name"
	// "k8s.deployment.uid"
	// "k8s.deployment.name"
	// "k8s.statefulset.uid"
	// "k8s.statefulset.name"
	// "k8s.daemonset.uid"
	// "k8s.daemonset.name"
	// "k8s.jobs.uid"
	// "k8s.jobs.name"
	// "k8s.cronjob.uid"
	// "k8s.cronjob.name"

	// resource/os
	"os.type":        "os.name",
	"os.description": "os.full",

	// resource/service
	// "service.name" -> Keep
	// "service.namespace" TODO
	// "service.version" -> Keep
	"service.instance.id": "service.id",

	// resource/telemetry TODO?
	// "telemetry.sdk.name"
	// "telemetry.sdk.language"
	// "telemetry.sdk.version"
	// "telemetry.auto.version"

	// TODO: ... more?
}
