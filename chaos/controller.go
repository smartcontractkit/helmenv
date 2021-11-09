package chaos

import (
	"bytes"
	"context"
	"fmt"
	"github.com/smartcontractkit/helmenv/chaos/experiments"
	"github.com/smartcontractkit/helmenv/tools"
	"io/ioutil"
	"path/filepath"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// APIBasePath in form of /apis/<spec.group>/<spec.versions.name>, see Chaosmesh CRD 2.0.0
	APIBasePath = "/apis/chaos-mesh.org/v1alpha1"
	// TemplatesPath path to the chaos templates
	TemplatesPath = "chaos/templates"
)

// Experimentable interface for chaos experiments
type Experimentable interface {
	SetBase(base experiments.Base)
	Filename() string
	Resource() string
}

// Controller is controller that manages Chaosmesh CRD instances to run experiments
type Controller struct {
	Client   *kubernetes.Clientset
	Requests map[string]*rest.Request
	Cfg      *Config
}

// Config Chaosmesh controller config
type Config struct {
	Client        *kubernetes.Clientset
	NamespaceName string
}

// ExperimentInfo persistent experiment info
type ExperimentInfo struct {
	Name     string `json:"name" mapstructure:"name"`
	Resource string `json:"resource" mapstructure:"resource"`
}

// NewController creates controller to run and stop chaos experiments
func NewController(cfg *Config) (*Controller, error) {
	return &Controller{
		Client:   cfg.Client,
		Requests: make(map[string]*rest.Request),
		Cfg:      cfg,
	}, nil
}

func (c *Controller) payloadFromStruct(exp Experimentable) (*CRDPayload, error) {
	name := fmt.Sprintf("%s-%s", exp.Resource(), uuid.NewV4().String())
	exp.SetBase(experiments.Base{
		Name:      name,
		Namespace: c.Cfg.NamespaceName,
	})
	fileBytes, err := ioutil.ReadFile(filepath.Join(tools.ProjectRoot, TemplatesPath, exp.Filename()))
	if err != nil {
		return nil, err
	}
	d, err := marshallTemplate(exp, "Chaos template", string(fileBytes))
	if err != nil {
		return nil, err
	}
	data, err := yaml.YAMLToJSON([]byte(d))
	if err != nil {
		return nil, err
	}
	return &CRDPayload{Name: name, Resource: exp.Resource(), Data: data}, nil
}

func (c *Controller) payloadFromTemplate(tmplPath string) (*CRDPayload, error) {
	tmplData, err := ioutil.ReadFile(tmplPath)
	if err != nil {
		return nil, err
	}
	var tmplMap map[string]interface{}
	if err := yaml.Unmarshal(tmplData, &tmplMap); err != nil {
		return nil, err
	}
	resource, ok := tmplMap["resource"].(string)
	if !ok {
		return nil, fmt.Errorf("chaos template must have 'resource' field, see Chaosmesh CRD resource types")
	}
	name := fmt.Sprintf("%s-%s", resource, uuid.NewV4().String())
	tmplMap["metadata"] = map[string]interface{}{
		"name":      name,
		"namespace": c.Cfg.NamespaceName,
	}
	d, err := yaml.Marshal(tmplMap)
	if err != nil {
		return nil, err
	}
	data, err := yaml.YAMLToJSON(d)
	if err != nil {
		return nil, err
	}
	return &CRDPayload{Name: name, Resource: resource, Data: data}, nil
}

// CRDPayload Custom Resource Defenition call payload
type CRDPayload struct {
	Name     string
	Resource string
	Data     []byte
}

// RunTemplate applies chaos from yaml template to a particular environment
func (c *Controller) RunTemplate(tmplPath string) (*ExperimentInfo, error) {
	payload, err := c.payloadFromTemplate(tmplPath)
	if err != nil {
		return nil, err
	}
	log.Info().
		Str("Name", payload.Name).
		Str("Resource", payload.Resource).
		Msg("Starting chaos experiment")
	req := c.Client.RESTClient().
		Post().
		AbsPath(APIBasePath).
		Name(payload.Name).
		Namespace(c.Cfg.NamespaceName).
		Resource(payload.Resource).
		Body(payload.Data)
	resp := req.Do(context.Background())
	if resp.Error() != nil {
		return nil, err
	}
	return &ExperimentInfo{Name: payload.Name, Resource: payload.Resource}, nil
}

// Run runs experiment and saves it's ID
func (c *Controller) Run(exp Experimentable) (string, error) {
	payload, err := c.payloadFromStruct(exp)
	if err != nil {
		return "", err
	}
	log.Info().
		Str("Name", payload.Name).
		Str("Resource", exp.Resource()).
		Msg("Starting chaos experiment")
	req := c.Client.RESTClient().
		Post().
		AbsPath(APIBasePath).
		Name(payload.Name).
		Namespace(c.Cfg.NamespaceName).
		Resource(exp.Resource()).
		Body(payload.Data)
	resp := req.Do(context.Background())
	if resp.Error() != nil {
		return "", err
	}
	c.Requests[payload.Name] = req
	return payload.Name, nil
}

// StopStandalone removes experiment's entity for standalone env
func (c *Controller) StopStandalone(expInfo *ExperimentInfo) error {
	log.Info().Str("ID", expInfo.Name).Msg("Deleting chaos experiment")
	req := c.Client.RESTClient().
		Delete().
		AbsPath(APIBasePath).
		Name(expInfo.Name).
		Resource(expInfo.Resource).
		Namespace(c.Cfg.NamespaceName)
	resp := req.Do(context.Background())
	if resp.Error() != nil {
		return resp.Error()
	}
	return resp.Error()
}

// Stop removes experiment's entity
func (c *Controller) Stop(name string) error {
	log.Info().Str("ID", name).Msg("Deleting chaos experiment")
	exp, ok := c.Requests[name]
	if !ok {
		return fmt.Errorf("experiment %s not found", name)
	}
	res := exp.Verb("DELETE").Do(context.Background())
	if res.Error() != nil {
		return res.Error()
	}
	delete(c.Requests, name)
	return nil
}

// StopAll removes all experiments entities
func (c *Controller) StopAll() error {
	for id := range c.Requests {
		err := c.Stop(id)
		if err != nil {
			return err
		}
	}
	return nil
}

func marshallTemplate(any interface{}, name, templateString string) (string, error) {
	var buf bytes.Buffer
	tmpl, err := template.New(name).Parse(templateString)
	if err != nil {
		return "", err
	}
	err = tmpl.Execute(&buf, any)
	if err != nil {
		return "", err
	}
	return buf.String(), err
}
