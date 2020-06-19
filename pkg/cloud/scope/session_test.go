package scope

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestPrincipalParsing(t *testing.T) {
	testCases := []struct {
		name string
		awsCluster infrav1.AWSCluster
		principalRef *corev1.ObjectReference
		principal    runtime.Object
		setup func(client.Client, *testing.T)
		expect func(credentials.Provider)
	}{
		{
			name: "Default case - no Principal specified",
			awsCluster: infrav1.AWSCluster{
				ObjectMeta: metav1.ObjectMeta {
					Name: "cluster1",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta {
					APIVersion: infrav1.GroupVersion.String(),
					Kind: "AWSCluster",
				},
				Spec: infrav1.AWSClusterSpec {},
			},
			setup: func(c client.Client, t *testing.T) {
			},
			expect: func(provider credentials.Provider) {
				if provider != nil {
					t.Fatalf("Expected provider to be nil, got: %v", provider)
				}
			},
		},
		{
			name: "Can get a session for a static Principal",
			awsCluster: infrav1.AWSCluster{
				ObjectMeta: metav1.ObjectMeta {
					Name: "cluster2",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta {
					APIVersion: infrav1.GroupVersion.String(),
					Kind: "AWSCluster",
				},
				Spec: infrav1.AWSClusterSpec {
					PrincipalRef: &corev1.ObjectReference{
						Name: "static-principal",
						Namespace: "default",
						Kind: "AWSClusterStaticPrincipal",
						APIVersion: infrav1.GroupVersion.String(),
					},
				},
			},
			setup: func(c client.Client, t *testing.T) {
				principal := &infrav1.AWSClusterStaticPrincipal {
					ObjectMeta: metav1.ObjectMeta{
						Name: "static-principal",
						Namespace: "default",
					},
					Spec: infrav1.AWSClusterStaticPrincipalSpec {
						SecretRef: corev1.SecretReference{
							Name: "static-credentials-secret",
							Namespace: "default",
						},
					},
				}
				principal.SetGroupVersionKind(infrav1.GroupVersion.WithKind("AWSClusterStaticPrincipal"))
				err := c.Create(context.Background(), principal)
				if err != nil {
					t.Fatal(err)
				}

				credentialsSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta {
						Name: "static-credentials-secret",
						Namespace: "default",
					},
					Data: map[string][]byte {
						"AccessKeyID": []byte("1234567890"),
						"SecretAccessKey": []byte("abcdefghijklmnop"),
						"SessionToken": []byte("asdfasdfasdf"),
					},
				}
				credentialsSecret.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Kind: "Secret", Version: "v1"})
				err = c.Create(context.Background(), credentialsSecret)
				if err != nil {
					t.Fatal(err)
				}
			},
			expect: func(provider credentials.Provider) {
				if provider == nil {
					t.Fatal("Expected provider not to be nil")
				}
				p, ok := provider.(*AWSStaticPrincipalTypeProvider)
				if !ok {
					t.Fatal("Expected provider to be of type AWSStaticPrincipalTypeProvider")
				}
				if p.accessKeyId != "1234567890" {
					t.Fatalf("Expected AccessKeyID to be '%s', got '%s'", "1234567890", p.accessKeyId)
				}
				if p.secretAccessKey != "abcdefghijklmnop" {
					t.Fatalf("Expected SecretAccessKey to be '%s', got '%s'", "abcdefghijklmnop", p.secretAccessKey)
				}
				if p.sessionToken != "asdfasdfasdf" {
					t.Fatalf("Expected SessionToken to be '%s', got '%s'", "asdfasdfasdf", p.sessionToken)
				}
			},
		},
		{
			name: "Can get a session for a role Principal",
			awsCluster: infrav1.AWSCluster{
				ObjectMeta: metav1.ObjectMeta {
					Name: "cluster3",
					Namespace: "default",
				},
				TypeMeta: metav1.TypeMeta {
					APIVersion: infrav1.GroupVersion.String(),
					Kind: "AWSCluster",
				},
				Spec: infrav1.AWSClusterSpec {
					PrincipalRef: &corev1.ObjectReference{
						Name: "role-principal",
						Namespace: "default",
						Kind: "AWSClusterRolePrincipal",
						APIVersion: infrav1.GroupVersion.String(),
					},
				},
			},
			setup: func(c client.Client, t *testing.T) {
				principal := &infrav1.AWSClusterRolePrincipal {
					ObjectMeta: metav1.ObjectMeta{
						Name: "role-principal",
						Namespace: "default",
					},
					Spec: infrav1.AWSClusterRolePrincipalSpec {
						AWSRoleSpec: infrav1.AWSRoleSpec{
							RoleArn: "role-arn",
						},
					},
				}
				principal.SetGroupVersionKind(infrav1.GroupVersion.WithKind("AWSClusterRolePrincipal"))
				err := c.Create(context.Background(), principal)
				if err != nil {
					t.Fatal(err)
				}
			},
			expect: func(provider credentials.Provider) {
				// session is a role Principal
				if provider == nil {
					t.Fatal("Expected provider not to be nil")
				}
				p, ok := provider.(*AWSRolePrincipalTypeProvider)
				if !ok {
					t.Fatal("Expected provider to be of type AWSRolePrincipalTypeProvider")
				}
				if p.Principal.Spec.RoleArn != "role-arn" {
					t.Fatal(fmt.Errorf("Expected Role Provider ARN to be 'role-arn', got '%s'", p.Principal.Spec.RoleArn))
				}
			},
		},
		// TODO: (andrewmy) ServiceAccountPrincipal not implemented yet
		//{
		//	name: "Can get a session for a service account Principal",
		//	awsCluster: infrav1.AWSCluster{
		//		ObjectMeta: metav1.ObjectMeta {
		//			Name: "cluster4",
		//			Namespace: "default",
		//		},
		//		TypeMeta: metav1.TypeMeta {
		//			APIVersion: infrav1.GroupVersion.String(),
		//			Kind: "AWSCluster",
		//		},
		//		Spec: infrav1.AWSClusterSpec {
		//			PrincipalRef: &corev1.ObjectReference{
		//				Name: "Principal",
		//				Kind: "AWSServiceAccountPrincipal",
		//			},
		//		},
		//	},
		//	setup: func(c client.Client, t *testing.T) {
		//		principal := &infrav1.AWSServiceAccountPrincipal {
		//			ObjectMeta: metav1.ObjectMeta{
		//				Name: "Principal",
		//			},
		//			Spec: infrav1.AWSServiceAccountPrincipalSpec {
		//				Audiences: []string{"audience-1", "audience-2"},
		//				AWSRoleSpec: infrav1.AWSRoleSpec{
		//					RoleArn: "role-arn",
		//				},
		//			},
		//		}
		//		principal.SetGroupVersionKind(infrav1.GroupVersion.WithKind("AWSClusterStaticPrincipal"))
		//		err := c.Create(context.Background(), principal)
		//		if err != nil {
		//			t.Fatal(err)
		//		}
		//
		//		serviceAccount := &corev1.ServiceAccount{
		//			ObjectMeta: metav1.ObjectMeta {
		//				Name: "test-service-account",
		//				Namespace: "default",
		//			},
		//		}
		//		err = c.Create(context.Background(), serviceAccount)
		//		if err != nil {
		//			t.Fatal(err)
		//		}
		//	},
		//	expect: func(provider credentials.Provider) {
		//		if provider == nil {
		//			t.Fatal("Expected provider not to be nil")
		//		}
		//		p, ok := provider.(*AWSServiceAccountPrincipalTypeProvider)
		//		if !ok {
		//			t.Fatal("Expected provider to be of type AWSRolePrincipalTypeProvider")
		//		}
		//		if len(p.Principal.Spec.Audiences) != 2 {
		//			t.Fatalf("Expected audiences to be a string array with 2 entries, got: %v", p.Principal.Spec.Audiences)
		//		}
		//		if p.Principal.Spec.Audiences[0] != "audience-1" {
		//			t.Fatalf("Expected audiences[0] to equal 'audience-1', got '%s'", p.Principal.Spec.Audiences[0])
		//		}
		//		if p.Principal.Spec.Audiences[1] != "audience-2" {
		//			t.Fatalf("Expected audiences[1] to equal 'audience-2', got '%s'", p.Principal.Spec.Audiences[1])
		//		}
		//		if p.Principal.Spec.RoleArn != "role-arn" {
		//			t.Fatalf("Expected role arn to be 'role-arn', got '%s'", p.Principal.Spec.RoleArn)
		//		}
		//	},
		//},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme, err := setupScheme()
			if err != nil {
				t.Fatal(err)
			}
			k8sClient := fake.NewFakeClientWithScheme(scheme)
			awsConfig := aws.NewConfig()
			tc.setup(k8sClient, t)
			provider, err := getProviderForCluster(context.Background(), k8sClient, &tc.awsCluster, awsConfig, klogr.New())
			if err != nil {
				t.Fatal(err)
			}
			tc.expect(provider)
		})
	}
}