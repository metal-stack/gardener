module github.com/gardener/gardener

go 1.20

require (
	github.com/Masterminds/semver v1.5.0
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/ahmetb/gen-crd-api-reference-docs v0.3.0
	github.com/bronze1man/yaml2json v0.0.0-20211227013850-8972abeaea25
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/fluent/fluent-operator/v2 v2.2.0
	github.com/gardener/dependency-watchdog v1.0.0
	github.com/gardener/etcd-druid v0.18.1
	github.com/gardener/hvpa-controller/api v0.5.0
	github.com/gardener/machine-controller-manager v0.48.1
	github.com/go-logr/logr v1.2.4
	github.com/go-test/deep v1.1.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.6.0
	github.com/google/gnostic v0.5.7-v3refs
	github.com/google/go-cmp v0.5.9
	github.com/google/go-containerregistry v0.15.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.2.0
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/onsi/ginkgo/v2 v2.9.2
	github.com/onsi/gomega v1.27.6
	github.com/prometheus/client_golang v1.14.0
	github.com/robfig/cron v1.2.0
	github.com/spf13/afero v1.8.2
	github.com/spf13/cobra v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.11.0
	github.com/texttheater/golang-levenshtein v1.0.1
	go.uber.org/automaxprocs v1.5.1
	go.uber.org/goleak v1.2.0
	go.uber.org/zap v1.24.0
	golang.org/x/crypto v0.6.0
	golang.org/x/exp v0.0.0-20230213192124-5e25df0256eb
	golang.org/x/text v0.9.0
	golang.org/x/time v0.3.0
	golang.org/x/tools v0.8.0
	gomodules.xyz/jsonpatch/v2 v2.2.0
	gonum.org/v1/gonum v0.12.0
	google.golang.org/protobuf v1.30.0
	gopkg.in/yaml.v2 v2.4.0
	istio.io/api v0.0.0-20230217221049-9d422bf48675
	istio.io/client-go v1.17.1
	k8s.io/api v0.26.3
	k8s.io/apiextensions-apiserver v0.26.3
	k8s.io/apimachinery v0.26.3
	k8s.io/apiserver v0.26.3
	k8s.io/autoscaler/vertical-pod-autoscaler v0.13.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cluster-bootstrap v0.26.3
	k8s.io/code-generator v0.26.3
	k8s.io/component-base v0.26.3
	k8s.io/component-helpers v0.26.3
	k8s.io/helm v2.16.1+incompatible
	k8s.io/klog/v2 v2.90.1
	k8s.io/kube-aggregator v0.26.3
	k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280
	k8s.io/kube-proxy v0.26.3
	k8s.io/kubelet v0.26.3
	k8s.io/metrics v0.26.3
	k8s.io/pod-security-admission v0.26.3
	k8s.io/utils v0.0.0-20221128185143-99ec85e7a448
	sigs.k8s.io/controller-runtime v0.14.6
	sigs.k8s.io/controller-runtime/tools/setup-envtest v0.0.0-20221212190805-d4f1e822ca11 // v0.14.1
	sigs.k8s.io/controller-tools v0.11.3
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr v1.4.10 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cenkalti/backoff/v4 v4.1.3 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/cli v23.0.5+incompatible // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v23.0.5+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/frankban/quicktest v1.14.4 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-logr/zapr v1.2.3 // indirect
	github.com/go-openapi/errors v0.20.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/swag v0.19.14 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gobuffalo/flect v0.3.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/godbus/dbus/v5 v5.0.4 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/cel-go v0.12.6 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/pprof v0.0.0-20210720184732-4bb14d4b1be1 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nwaples/rardecode v1.1.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc3 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pelletier/go-toml/v2 v2.0.0-beta.8 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/ulikunitz/xz v0.5.10 // indirect
	github.com/vbatts/tar-split v0.11.3 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	go.etcd.io/etcd/api/v3 v3.5.5 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.5 // indirect
	go.etcd.io/etcd/client/v3 v3.5.5 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.35.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.35.0 // indirect
	go.opentelemetry.io/otel v1.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.10.0 // indirect
	go.opentelemetry.io/otel/metric v0.31.0 // indirect
	go.opentelemetry.io/otel/sdk v1.10.0 // indirect
	go.opentelemetry.io/otel/trace v1.10.0 // indirect
	go.opentelemetry.io/proto/otlp v0.19.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/oauth2 v0.7.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/term v0.7.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20221018160656-63c7b68cfc55 // indirect
	google.golang.org/grpc v1.50.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.66.4 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/gengo v0.0.0-20220902162205-c0856e24416d // indirect
	k8s.io/klog v1.0.0 // indirect
	k8s.io/kms v0.26.3 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.36 // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)

replace k8s.io/client-go => k8s.io/client-go v0.26.3
