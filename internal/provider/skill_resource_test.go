package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func writeSkillDir(t *testing.T, root, extra string) string {
	t.Helper()
	dir := filepath.Join(root, "example-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: example-skill\ndescription: Formats release notes from a changelog.\n---\n# Example skill\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "template.md"), []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAccSkillResource(t *testing.T) {
	root := t.TempDir()
	dir := writeSkillDir(t, root, "## {{version}}\n")
	config := providerConfig + fmt.Sprintf(`
resource "anthropic_skill" "test" {
  source_dir = %q
}
data "anthropic_skill" "read"              { id = anthropic_skill.test.id }
data "anthropic_skills" "custom"           { source = "custom" }
data "anthropic_skill_versions" "versions" { skill_id = anthropic_skill.test.id }
`, dir)
	var firstVersion string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_skill.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^skill_`))),
					statecheck.ExpectKnownValue("anthropic_skill.test", tfjsonpath.New("display_name"), knownvalue.StringExact("example-skill")),
					statecheck.ExpectKnownValue("anthropic_skill.test", tfjsonpath.New("name"), knownvalue.StringExact("example-skill")),
					statecheck.ExpectKnownValue("anthropic_skill.test", tfjsonpath.New("source_type"), knownvalue.StringExact("custom")),
					statecheck.ExpectKnownValue("anthropic_skill.test", tfjsonpath.New("content_hash"), knownvalue.StringRegexp(regexp.MustCompile(`^[0-9a-f]{64}$`))),
					statecheck.ExpectKnownValue("data.anthropic_skill.read", tfjsonpath.New("latest_version_name"), knownvalue.StringExact("example-skill")),
				},
				Check: resource.TestCheckResourceAttrWith("anthropic_skill.test", "latest_version_id", func(v string) error {
					firstVersion = v
					return nil
				}),
			},
			{Config: config, PlanOnly: true},
			{
				PreConfig: func() { writeSkillDir(t, root, "## {{version}} changed\n") },
				Config:    config,
				Check: resource.TestCheckResourceAttrWith("anthropic_skill.test", "latest_version_id", func(v string) error {
					if v == firstVersion || v == "" {
						return fmt.Errorf("expected a new version id after content change, got %q", v)
					}
					return nil
				}),
			},
			{
				ResourceName:            "anthropic_skill.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source_dir", "content_hash", "delete_on_destroy"},
			},
		},
	})
}

func TestAccSkillResource_missingSkillMD(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "broken")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("no frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      providerConfig + fmt.Sprintf(`resource "anthropic_skill" "bad" { source_dir = %q }`, dir),
			ExpectError: regexp.MustCompile(`must contain SKILL.md`),
		}},
	})
}
