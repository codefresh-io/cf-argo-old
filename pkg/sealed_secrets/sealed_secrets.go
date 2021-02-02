package sealed_secrets

import (
	"bytes"
	"context"
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"

	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"github.com/codefresh-io/cf-argo/pkg/store"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubectl/pkg/scheme"
)

// GroupName is the group name used in this package

const (
	GroupName = "bitnami.com"
	// SealedSecretName is the name used in SealedSecret CRD
	SealedSecretName = "sealed-secret." + GroupName
	// SealedSecretPlural is the collection plural used with SealedSecret API
	SealedSecretPlural = "sealedsecrets"

	// Annotation namespace prefix
	annoNs = "sealedsecrets." + GroupName + "/"

	// SealedSecretClusterWideAnnotation is the name for the annotation for
	// setting the secret to be available cluster wide.
	SealedSecretClusterWideAnnotation = annoNs + "cluster-wide"

	// SealedSecretNamespaceWideAnnotation is the name for the annotation for
	// setting the secret to be available namespace wide.
	SealedSecretNamespaceWideAnnotation = annoNs + "namespace-wide"

	// SealedSecretManagedAnnotation is the name for the annotation for
	// flaging the existing secrets be managed by SealedSecret controller.
	SealedSecretManagedAnnotation = annoNs + "managed"
)

// SecretTemplateSpec describes the structure a Secret should have
// when created from a template
type SecretTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Used to facilitate programmatic handling of secret data.
	// +optional
	Type apiv1.SecretType `json:"type,omitempty" protobuf:"bytes,3,opt,name=type,casttype=SecretType"`
}

// SealedSecretSpec is the specification of a SealedSecret
type SealedSecretSpec struct {
	// Template defines the structure of the Secret that will be
	// created from this sealed secret.
	// +optional
	Template SecretTemplateSpec `json:"template,omitempty"`

	// Data is deprecated and will be removed eventually. Use per-value EncryptedData instead.
	Data          []byte            `json:"data,omitempty"`
	EncryptedData map[string]string `json:"encryptedData"`
}

// SealedSecretConditionType describes the type of SealedSecret condition
type SealedSecretConditionType string

const (
	// SealedSecretSynced means the SealedSecret has been decrypted and the Secret has been updated successfully.
	SealedSecretSynced SealedSecretConditionType = "Synced"
)

// SealedSecretCondition describes the state of a sealed secret at a certain point.
type SealedSecretCondition struct {
	// Type of condition for a sealed secret.
	// Valid value: "Synced"
	Type SealedSecretConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=DeploymentConditionType"`
	// Status of the condition for a sealed secret.
	// Valid values for "Synced": "True", "False", or "Unknown".
	Status apiv1.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/api/core/v1.ConditionStatus"`
	// The last time this condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty" protobuf:"bytes,6,opt,name=lastUpdateTime"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,7,opt,name=lastTransitionTime"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty" protobuf:"bytes,5,opt,name=message"`
}

// SealedSecretStatus is the most recently observed status of the SealedSecret.
type SealedSecretStatus struct {
	// ObservedGeneration reflects the generation most recently observed by the sealed-secrets controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,3,opt,name=observedGeneration"`

	// Represents the latest available observations of a sealed secret's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []SealedSecretCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,6,rep,name=conditions"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient

// SealedSecret is the K8s representation of a "sealed Secret" - a
// regular k8s Secret that has been sealed (encrypted) using the
// controller's key.
type SealedSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SealedSecretSpec `json:"spec"`
	// +optional
	Status *SealedSecretStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SealedSecretList represents a list of SealedSecrets
type SealedSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SealedSecret `json:"items"`
}

// ByCreationTimestamp is used to sort a list of secrets
type ByCreationTimestamp []apiv1.Secret

func (s ByCreationTimestamp) Len() int {
	return len(s)
}

func (s ByCreationTimestamp) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s ByCreationTimestamp) Less(i, j int) bool {
	return s[i].GetCreationTimestamp().Unix() < s[j].GetCreationTimestamp().Unix()
}

func CreateSealedSecret(ctx context.Context, secretPath string) error {
	conf, err := store.Get().NewKubeClient(ctx).ToRESTConfig()
	if err != nil {
		return err
	}

	restClient, err := corev1.NewForConfig(conf)
	if err != nil {
		return err
	}

	f, err := restClient.
		Services("envname-argocd").
		ProxyGet("http", "sealed-secrets-controller", "", "/v1/cert.pem", nil).
		Stream(ctx)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, f)
	block, _ := pem.Decode(buf)
	if block == nil {
		panic("failed to parse PEM block containing the public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic("failed to parse DER encoded public key: " + err.Error())
	}

	switch pub := pub.(type) {
	case *rsa.PublicKey:
		fmt.Println("pub is of type RSA:", pub)
	case *dsa.PublicKey:
		fmt.Println("pub is of type DSA:", pub)
	case *ecdsa.PublicKey:
		fmt.Println("pub is of type ECDSA:", pub)
	default:
		panic("unknown type of public key")
	}

	sealedSecret, err := ssApi.NewSealedSecret(scheme.Codecs, nil /*rsa.publickey*/, nil /*secret*/)

	sealedSecret, err = ssClient.SealedSecrets("some-namespace").Create(sealedSecret)
	return nil
}

func NewSealedSecret(codecs runtimeserializer.CodecFactory, pubKey *rsa.PublicKey, secret *v1.Secret) (*SealedSecret, error) {
	// if SecretScope(secret) != ClusterWideScope && secret.GetNamespace() == "" {
	// 	return nil, fmt.Errorf("secret must declare a namespace")
	// }

	s := &SealedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.GetName(),
			Namespace: secret.GetNamespace(),
		},
		Spec: SealedSecretSpec{
			Template: SecretTemplateSpec{
				// ObjectMeta copied below
				Type: secret.Type,
			},
			EncryptedData: map[string]string{},
		},
	}
	secret.ObjectMeta.DeepCopyInto(&s.Spec.Template.ObjectMeta)

	// the input secret could come from a real secret object applied with `kubectl apply` or similar tools
	// which put a copy of the object version at application time in an annotation in order to support
	// strategic merge patch in subsequent updates. We need to strip those annotations or else we would
	// be leaking secrets in clear in a way that might be non obvious to users.
	// See https://github.com/bitnami-labs/sealed-secrets/issues/227
	StripLastAppliedAnnotations(s.Spec.Template.ObjectMeta.Annotations)

	// Cleanup ownerReference (See #243)
	s.Spec.Template.ObjectMeta.OwnerReferences = nil

	// RSA-OAEP will fail to decrypt unless the same label is used
	// during decryption.
	label := labelFor(secret)

	for key, value := range secret.Data {
		ciphertext, err := crypto.HybridEncrypt(rand.Reader, pubKey, value, label)
		if err != nil {
			return nil, err
		}
		s.Spec.EncryptedData[key] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	for key, value := range secret.StringData {
		ciphertext, err := crypto.HybridEncrypt(rand.Reader, pubKey, []byte(value), label)
		if err != nil {
			return nil, err
		}
		s.Spec.EncryptedData[key] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	s.Annotations = UpdateScopeAnnotations(s.Annotations, SecretScope(secret))

	return s, nil
}
