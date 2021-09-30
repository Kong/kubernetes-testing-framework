package httpbin

// -----------------------------------------------------------------------------
// Kong Addon - Builder
// -----------------------------------------------------------------------------

// Builder is a configuration tool to generate HttpBin cluster addons.
type Builder struct {
	name               string
	namespace          string
	generateNamespace  bool
	ingressAnnotations map[string]string
}

// NewBuilder provides a new Builder object for configuring HttpBin cluster addons.
func NewBuilder() *Builder {
	return &Builder{
		name:               string(AddonName),
		namespace:          DefaultNamespace,
		ingressAnnotations: make(map[string]string),
	}
}

// WithName indicates the name of the Addon which is useful if the caller intends
// to deploy multiple copies of the addon into a single namespace.
func (b *Builder) WithName(name string) *Builder {
	b.name = name
	return b
}

// WithNamespace allows the namespace where the addon should be deployed to be
// overridden from the default.
func (b *Builder) WithNamespace(namespace string) *Builder {
	b.namespace = namespace
	return b
}

// WithGeneratedNamespace indicates that a uniquely named namespace should be
// used. Helpful when deploying multiple copies of HttpBin to the cluster.
func (b *Builder) WithGeneratedNamespace() *Builder {
	b.generateNamespace = true
	return b
}

// WithIngressAnnotations allows injecting the annotations that will be placed
// on the HttpBin Ingress resource for things like deciding the ingress.class.
// This will override values for new keys provided, but will combine with any
// other previously existing keys.
func (b *Builder) WithIngressAnnotations(anns map[string]string) *Builder {
	for k, v := range anns {
		b.ingressAnnotations[k] = v
	}
	return b
}

// Build generates a new kong cluster.Addon which can be loaded and deployed
// into a test Environment's cluster.Cluster.
func (b *Builder) Build() *Addon {
	// if no ingress.class annotations are provided we'll assume Kong should
	// be the ingress class.
	if _, ok := b.ingressAnnotations["networking.knative.dev/ingress.class"]; !ok {
		b.ingressAnnotations["networking.knative.dev/ingress.class"] = "kong"
	}
	if _, ok := b.ingressAnnotations["kubernetes.io/ingress.class"]; !ok {
		b.ingressAnnotations["kubernetes.io/ingress.class"] = "kong"
	}
	if _, ok := b.ingressAnnotations["konghq.com/strip-path"]; !ok {
		b.ingressAnnotations["konghq.com/strip-path"] = "true"
	}

	return &Addon{
		name:               b.name,
		namespace:          b.namespace,
		generateNamespace:  b.generateNamespace,
		ingressAnnotations: b.ingressAnnotations,
	}
}
