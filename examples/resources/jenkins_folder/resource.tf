resource "jenkins_folder" "parent" {
  name        = "team"
  description = "Jobs owned by the team"
}

# Nested folder, created at team/project
resource "jenkins_folder" "child" {
  name   = "project"
  folder = jenkins_folder.parent.id
}
