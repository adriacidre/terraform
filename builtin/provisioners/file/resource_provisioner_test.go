package file

import (
	"testing"

	"github.com/r3labs/terraform/config"
	"github.com/r3labs/terraform/terraform"
)

func TestResourceProvider_Validate_good_source(t *testing.T) {
	c := testConfig(t, map[string]interface{}{
		"source":      "/tmp/foo",
		"destination": "/tmp/bar",
	})
	p := Provisioner()
	warn, errs := p.Validate(c)
	if len(warn) > 0 {
		t.Fatalf("Warnings: %v", warn)
	}
	if len(errs) > 0 {
		t.Fatalf("Errors: %v", errs)
	}
}

func TestResourceProvider_Validate_good_content(t *testing.T) {
	c := testConfig(t, map[string]interface{}{
		"content":     "value to copy",
		"destination": "/tmp/bar",
	})
	p := Provisioner()
	warn, errs := p.Validate(c)
	if len(warn) > 0 {
		t.Fatalf("Warnings: %v", warn)
	}
	if len(errs) > 0 {
		t.Fatalf("Errors: %v", errs)
	}
}

func TestResourceProvider_Validate_bad_not_destination(t *testing.T) {
	c := testConfig(t, map[string]interface{}{
		"source": "nope",
	})
	p := Provisioner()
	warn, errs := p.Validate(c)
	if len(warn) > 0 {
		t.Fatalf("Warnings: %v", warn)
	}
	if len(errs) == 0 {
		t.Fatalf("Should have errors")
	}
}

func TestResourceProvider_Validate_bad_to_many_src(t *testing.T) {
	c := testConfig(t, map[string]interface{}{
		"source":      "nope",
		"content":     "value to copy",
		"destination": "/tmp/bar",
	})
	p := Provisioner()
	warn, errs := p.Validate(c)
	if len(warn) > 0 {
		t.Fatalf("Warnings: %v", warn)
	}
	if len(errs) == 0 {
		t.Fatalf("Should have errors")
	}
}

func testConfig(
	t *testing.T,
	c map[string]interface{}) *terraform.ResourceConfig {
	r, err := config.NewRawConfig(c)
	if err != nil {
		t.Fatalf("bad: %s", err)
	}
	return terraform.NewResourceConfig(r)
}
