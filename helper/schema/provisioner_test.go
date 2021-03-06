package schema

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/r3labs/terraform/config"
	"github.com/r3labs/terraform/terraform"
)

func TestProvisioner_impl(t *testing.T) {
	var _ terraform.ResourceProvisioner = new(Provisioner)
}

func TestProvisionerValidate(t *testing.T) {
	cases := []struct {
		Name   string
		P      *Provisioner
		Config map[string]interface{}
		Err    bool
	}{
		{
			"Basic required field",
			&Provisioner{
				Schema: map[string]*Schema{
					"foo": &Schema{
						Required: true,
						Type:     TypeString,
					},
				},
			},
			nil,
			true,
		},

		{
			"Basic required field set",
			&Provisioner{
				Schema: map[string]*Schema{
					"foo": &Schema{
						Required: true,
						Type:     TypeString,
					},
				},
			},
			map[string]interface{}{
				"foo": "bar",
			},
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.Name), func(t *testing.T) {
			c, err := config.NewRawConfig(tc.Config)
			if err != nil {
				t.Fatalf("err: %s", err)
			}

			_, es := tc.P.Validate(terraform.NewResourceConfig(c))
			if len(es) > 0 != tc.Err {
				t.Fatalf("%d: %#v", i, es)
			}
		})
	}
}

func TestProvisionerApply(t *testing.T) {
	cases := []struct {
		Name   string
		P      *Provisioner
		Conn   map[string]string
		Config map[string]interface{}
		Err    bool
	}{
		{
			"Basic config",
			&Provisioner{
				ConnSchema: map[string]*Schema{
					"foo": &Schema{
						Type:     TypeString,
						Optional: true,
					},
				},

				Schema: map[string]*Schema{
					"foo": &Schema{
						Type:     TypeInt,
						Optional: true,
					},
				},

				ApplyFunc: func(ctx context.Context) error {
					cd := ctx.Value(ProvConnDataKey).(*ResourceData)
					d := ctx.Value(ProvConfigDataKey).(*ResourceData)
					if d.Get("foo").(int) != 42 {
						return fmt.Errorf("bad config data")
					}
					if cd.Get("foo").(string) != "bar" {
						return fmt.Errorf("bad conn data")
					}

					return nil
				},
			},
			map[string]string{
				"foo": "bar",
			},
			map[string]interface{}{
				"foo": 42,
			},
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.Name), func(t *testing.T) {
			c, err := config.NewRawConfig(tc.Config)
			if err != nil {
				t.Fatalf("err: %s", err)
			}

			state := &terraform.InstanceState{
				Ephemeral: terraform.EphemeralState{
					ConnInfo: tc.Conn,
				},
			}

			err = tc.P.Apply(
				nil, state, terraform.NewResourceConfig(c))
			if err != nil != tc.Err {
				t.Fatalf("%d: %s", i, err)
			}
		})
	}
}

func TestProvisionerApply_nilState(t *testing.T) {
	p := &Provisioner{
		ConnSchema: map[string]*Schema{
			"foo": &Schema{
				Type:     TypeString,
				Optional: true,
			},
		},

		Schema: map[string]*Schema{
			"foo": &Schema{
				Type:     TypeInt,
				Optional: true,
			},
		},

		ApplyFunc: func(ctx context.Context) error {
			return nil
		},
	}

	conf := map[string]interface{}{
		"foo": 42,
	}

	c, err := config.NewRawConfig(conf)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = p.Apply(nil, nil, terraform.NewResourceConfig(c))
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvisionerStop(t *testing.T) {
	var p Provisioner

	// Verify stopch blocks
	ch := p.StopContext().Done()
	select {
	case <-ch:
		t.Fatal("should not be stopped")
	case <-time.After(10 * time.Millisecond):
	}

	// Stop it
	if err := p.Stop(); err != nil {
		t.Fatalf("err: %s", err)
	}

	select {
	case <-ch:
	case <-time.After(10 * time.Millisecond):
		t.Fatal("should be stopped")
	}
}

func TestProvisionerStop_apply(t *testing.T) {
	p := &Provisioner{
		ConnSchema: map[string]*Schema{
			"foo": &Schema{
				Type:     TypeString,
				Optional: true,
			},
		},

		Schema: map[string]*Schema{
			"foo": &Schema{
				Type:     TypeInt,
				Optional: true,
			},
		},

		ApplyFunc: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
	}

	conn := map[string]string{
		"foo": "bar",
	}

	conf := map[string]interface{}{
		"foo": 42,
	}

	c, err := config.NewRawConfig(conf)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	state := &terraform.InstanceState{
		Ephemeral: terraform.EphemeralState{
			ConnInfo: conn,
		},
	}

	// Run the apply in a goroutine
	doneCh := make(chan struct{})
	go func() {
		p.Apply(nil, state, terraform.NewResourceConfig(c))
		close(doneCh)
	}()

	// Should block
	select {
	case <-doneCh:
		t.Fatal("should not be done")
	case <-time.After(10 * time.Millisecond):
	}

	// Stop!
	p.Stop()

	select {
	case <-doneCh:
	case <-time.After(10 * time.Millisecond):
		t.Fatal("should be done")
	}
}

func TestProvisionerStop_stopFirst(t *testing.T) {
	var p Provisioner

	// Stop it
	if err := p.Stop(); err != nil {
		t.Fatalf("err: %s", err)
	}

	select {
	case <-p.StopContext().Done():
	case <-time.After(10 * time.Millisecond):
		t.Fatal("should be stopped")
	}
}
