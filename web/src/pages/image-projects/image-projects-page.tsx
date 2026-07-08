import { type Dispatch, type FormEvent, type ReactNode, type SetStateAction, useMemo, useState } from "react"
import {
  Archive,
  Boxes,
  GitBranch,
  GitCommit,
  Loader2,
  Plus,
  Search,
} from "lucide-react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { Button } from "@/components/ui/button"
import {
  archiveBranch,
  archiveImageProject,
  createBranch,
  createImageProject,
  createVersionNode,
  getVersionGraph,
  listImageProjects,
  type ImageType,
  type ProjectInput,
  type VersionGraph,
  type VersionNode,
} from "@/lib/image-projects-api"
import { cn } from "@/lib/utils"

type ProjectFormState = {
  name: string
  imageType: ImageType
  imageName: string
  namespace: string
  rootImageRef: string
  defaultArchitecture: string
  labels: string
  description: string
}

type BranchFormState = {
  name: string
  startNodeId: string
  description: string
}

type NodeFormState = {
  branchId: string
  parentNodeId: string
  version: string
  dockerfile: string
  description: string
}

const defaultProjectForm: ProjectFormState = {
  name: "Java Runtime",
  imageType: "java",
  imageName: "java-runtime",
  namespace: "platform",
  rootImageRef: "eclipse-temurin:17",
  defaultArchitecture: "amd64",
  labels: "java,jdk17",
  description: "",
}

export function ImageProjectsPage() {
  const queryClient = useQueryClient()
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(null)
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [showProjectForm, setShowProjectForm] = useState(false)
  const [projectForm, setProjectForm] = useState(defaultProjectForm)
  const [projectError, setProjectError] = useState("")
  const [branchForm, setBranchForm] = useState<BranchFormState>({ name: "", startNodeId: "", description: "" })
  const [nodeForm, setNodeForm] = useState<NodeFormState>({
    branchId: "",
    parentNodeId: "",
    version: "",
    dockerfile: "",
    description: "",
  })
  const [keyword, setKeyword] = useState("")

  const projectsQuery = useQuery({
    queryKey: ["image-projects"],
    queryFn: listImageProjects,
  })

  const selectedProject = useMemo(() => {
    const projects = projectsQuery.data ?? []
    return projects.find((project) => project.id === selectedProjectId) ?? projects[0] ?? null
  }, [projectsQuery.data, selectedProjectId])

  const graphQuery = useQuery({
    queryKey: ["image-projects", selectedProject?.id, "graph"],
    queryFn: () => getVersionGraph(selectedProject!.id),
    enabled: selectedProject != null,
  })

  const selectedNode = useMemo(() => {
    const nodes = graphQuery.data?.nodes ?? []
    return nodes.find((node) => node.id === selectedNodeId) ?? nodes[0] ?? null
  }, [graphQuery.data?.nodes, selectedNodeId])

  const filteredProjects = useMemo(() => {
    const value = keyword.trim().toLowerCase()
    const projects = projectsQuery.data ?? []
    if (!value) {
      return projects
    }
    return projects.filter((project) =>
      [project.name, project.imageName, project.rootImageRef, project.imageType].some((field) =>
        field.toLowerCase().includes(value),
      ),
    )
  }, [projectsQuery.data, keyword])

  const createProjectMutation = useMutation({
    mutationFn: createImageProject,
    onSuccess: (project) => {
      setSelectedProjectId(project.id)
      setShowProjectForm(false)
      setProjectForm(defaultProjectForm)
      return queryClient.invalidateQueries({ queryKey: ["image-projects"] })
    },
  })

  const archiveProjectMutation = useMutation({
    mutationFn: archiveImageProject,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["image-projects"] }),
  })

  const createBranchMutation = useMutation({
    mutationFn: ({ projectId, input }: { projectId: string; input: BranchFormState }) =>
      createBranch(projectId, {
        name: input.name,
        startNodeId: input.startNodeId,
        description: input.description,
      }),
    onSuccess: () => {
      setBranchForm({ name: "", startNodeId: "", description: "" })
      return queryClient.invalidateQueries({ queryKey: ["image-projects", selectedProject?.id, "graph"] })
    },
  })

  const archiveBranchMutation = useMutation({
    mutationFn: ({ projectId, branchId }: { projectId: string; branchId: string }) => archiveBranch(projectId, branchId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["image-projects", selectedProject?.id, "graph"] }),
  })

  const createNodeMutation = useMutation({
    mutationFn: ({ projectId, input }: { projectId: string; input: NodeFormState }) =>
      createVersionNode(projectId, {
        branchId: input.branchId,
        parentNodeId: input.parentNodeId,
        version: input.version,
        dockerfile: input.dockerfile,
        description: input.description,
        status: "active",
      }),
    onSuccess: (node) => {
      setSelectedNodeId(node.id)
      setNodeForm({ branchId: "", parentNodeId: "", version: "", dockerfile: "", description: "" })
      return queryClient.invalidateQueries({ queryKey: ["image-projects", selectedProject?.id, "graph"] })
    },
  })

  async function handleCreateProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setProjectError("")
    try {
      await createProjectMutation.mutateAsync(toProjectInput(projectForm))
    } catch (error) {
      setProjectError(error instanceof Error ? error.message : "创建镜像项目失败。")
    }
  }

  function prepareBranchFromNode(node: VersionNode) {
    setBranchForm({
      name: `${node.version}-branch`,
      startNodeId: node.id,
      description: "",
    })
  }

  function prepareVersionFromNode(node: VersionNode, graph: VersionGraph) {
    setNodeForm({
      branchId: node.branchId || graph.branches[0]?.id || "",
      parentNodeId: node.id,
      version: `${node.version}-next`,
      dockerfile: node.dockerfile,
      description: "",
    })
  }

  return (
    <div className="grid gap-6 xl:grid-cols-[360px_1fr]">
      <aside className="space-y-4">
        <section className="rounded-lg border bg-card p-4 text-card-foreground">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold">Image Projects</h2>
              <p className="mt-1 text-sm text-muted-foreground">Select a project before viewing its version line.</p>
            </div>
            <Button size="sm" onClick={() => setShowProjectForm((value) => !value)}>
              <Plus aria-hidden="true" />
              Create
            </Button>
          </div>

          <label className="mt-4 flex h-10 items-center gap-2 rounded-md border bg-background px-3 text-sm">
            <Search className="size-4 text-muted-foreground" aria-hidden="true" />
            <input
              className="min-w-0 flex-1 bg-transparent outline-none"
              placeholder="Search projects"
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
            />
          </label>

          {showProjectForm ? (
            <form className="mt-4 space-y-3 rounded-md border bg-background p-3" onSubmit={handleCreateProject}>
              <Field label="Project Name">
                <input className={inputClassName} value={projectForm.name} onChange={(event) => updateForm(setProjectForm, "name", event.target.value)} required />
              </Field>
              <Field label="Image Type">
                <select className={inputClassName} value={projectForm.imageType} onChange={(event) => updateForm(setProjectForm, "imageType", event.target.value)}>
                  <option value="java">Java</option>
                  <option value="python">Python</option>
                  <option value="nodejs">Node.js</option>
                  <option value="mysql">MySQL</option>
                  <option value="base_os">Base OS</option>
                  <option value="other">Other</option>
                </select>
              </Field>
              <Field label="Image Name">
                <input className={inputClassName} value={projectForm.imageName} onChange={(event) => updateForm(setProjectForm, "imageName", event.target.value)} required />
              </Field>
              <Field label="Namespace">
                <input className={inputClassName} value={projectForm.namespace} onChange={(event) => updateForm(setProjectForm, "namespace", event.target.value)} />
              </Field>
              <Field label="Root Image">
                <input className={inputClassName} value={projectForm.rootImageRef} onChange={(event) => updateForm(setProjectForm, "rootImageRef", event.target.value)} required />
              </Field>
              <Field label="Default Architecture">
                <input className={inputClassName} value={projectForm.defaultArchitecture} onChange={(event) => updateForm(setProjectForm, "defaultArchitecture", event.target.value)} required />
              </Field>
              <Field label="Labels">
                <input className={inputClassName} value={projectForm.labels} onChange={(event) => updateForm(setProjectForm, "labels", event.target.value)} />
              </Field>
              <Field label="Description">
                <textarea
                  className="min-h-20 w-full rounded-md border bg-background px-3 py-2 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
                  value={projectForm.description}
                  onChange={(event) => updateForm(setProjectForm, "description", event.target.value)}
                />
              </Field>
              {projectError ? <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{projectError}</div> : null}
              <Button className="w-full" type="submit" disabled={createProjectMutation.isPending}>
                {createProjectMutation.isPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create Project
              </Button>
            </form>
          ) : null}
        </section>

        <section className="rounded-lg border bg-card text-card-foreground">
          {projectsQuery.isPending ? (
            <StateBlock title="Loading projects" detail="Fetching image project catalog." />
          ) : projectsQuery.isError ? (
            <StateBlock title="Failed to load projects" detail="Please retry after checking the backend service." />
          ) : filteredProjects.length === 0 ? (
            <StateBlock title="No projects" detail="Create a Java, Python, Node.js, MySQL, or base image project." />
          ) : (
            <div className="divide-y">
              {filteredProjects.map((project) => (
                <button
                  key={project.id}
                  type="button"
                  className={cn(
                    "block w-full px-4 py-3 text-left transition-colors hover:bg-accent",
                    selectedProject?.id === project.id && "bg-accent",
                  )}
                  onClick={() => {
                    setSelectedProjectId(project.id)
                    setSelectedNodeId(project.latestVersionNodeId ?? null)
                  }}
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium">{project.name}</div>
                      <div className="mt-1 truncate text-xs text-muted-foreground">
                        {project.namespace ? `${project.namespace}/` : ""}
                        {project.imageName}
                      </div>
                    </div>
                    <span className="rounded-md bg-secondary px-2 py-1 text-xs text-secondary-foreground">
                      {project.imageType}
                    </span>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-1">
                    <Pill>{project.defaultArchitecture}</Pill>
                    {project.latestVersion ? <Pill>{project.latestVersion}</Pill> : null}
                  </div>
                </button>
              ))}
            </div>
          )}
        </section>
      </aside>

      <main className="space-y-4">
        {!selectedProject ? (
          <section className="rounded-lg border bg-card p-8 text-center text-card-foreground">
            <Boxes className="mx-auto size-8 text-muted-foreground" aria-hidden="true" />
            <h2 className="mt-3 text-base font-semibold">No Project Selected</h2>
            <p className="mt-1 text-sm text-muted-foreground">Create or select a project to view its version branches.</p>
          </section>
        ) : graphQuery.isPending ? (
          <section className="rounded-lg border bg-card p-8 text-center text-card-foreground">
            <Loader2 className="mx-auto size-6 animate-spin text-muted-foreground" aria-hidden="true" />
            <p className="mt-3 text-sm text-muted-foreground">Loading version graph.</p>
          </section>
        ) : graphQuery.isError ? (
          <section className="rounded-lg border bg-card p-8 text-center text-card-foreground">
            <h2 className="text-base font-semibold">Failed to Load Graph</h2>
            <p className="mt-1 text-sm text-muted-foreground">Please retry after checking the backend service.</p>
          </section>
        ) : (
          <GraphWorkspace
            graph={graphQuery.data}
            selectedNode={selectedNode}
            branchForm={branchForm}
            nodeForm={nodeForm}
            createBranchPending={createBranchMutation.isPending}
            createNodePending={createNodeMutation.isPending}
            archiveProjectPending={archiveProjectMutation.isPending}
            archiveBranchPending={archiveBranchMutation.isPending}
            onSelectNode={setSelectedNodeId}
            onPrepareBranch={prepareBranchFromNode}
            onPrepareVersion={prepareVersionFromNode}
            onBranchFormChange={setBranchForm}
            onNodeFormChange={setNodeForm}
            onCreateBranch={() => {
              if (selectedProject) {
                createBranchMutation.mutate({ projectId: selectedProject.id, input: branchForm })
              }
            }}
            onCreateNode={() => {
              if (selectedProject) {
                createNodeMutation.mutate({ projectId: selectedProject.id, input: nodeForm })
              }
            }}
            onArchiveProject={() => archiveProjectMutation.mutate(selectedProject.id)}
            onArchiveBranch={(branchId) => archiveBranchMutation.mutate({ projectId: selectedProject.id, branchId })}
          />
        )}
      </main>
    </div>
  )
}

function GraphWorkspace({
  graph,
  selectedNode,
  branchForm,
  nodeForm,
  createBranchPending,
  createNodePending,
  archiveProjectPending,
  archiveBranchPending,
  onSelectNode,
  onPrepareBranch,
  onPrepareVersion,
  onBranchFormChange,
  onNodeFormChange,
  onCreateBranch,
  onCreateNode,
  onArchiveProject,
  onArchiveBranch,
}: {
  graph: VersionGraph
  selectedNode: VersionNode | null
  branchForm: BranchFormState
  nodeForm: NodeFormState
  createBranchPending: boolean
  createNodePending: boolean
  archiveProjectPending: boolean
  archiveBranchPending: boolean
  onSelectNode: (nodeId: string) => void
  onPrepareBranch: (node: VersionNode) => void
  onPrepareVersion: (node: VersionNode, graph: VersionGraph) => void
  onBranchFormChange: Dispatch<SetStateAction<BranchFormState>>
  onNodeFormChange: Dispatch<SetStateAction<NodeFormState>>
  onCreateBranch: () => void
  onCreateNode: () => void
  onArchiveProject: () => void
  onArchiveBranch: (branchId: string) => void
}) {
  const activeBranches = graph.branches.filter((branch) => branch.status === "active")

  return (
    <>
      <section className="rounded-lg border bg-card p-5 text-card-foreground">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-base font-semibold">{graph.project.name}</h2>
              <Pill>{graph.project.imageType}</Pill>
              <Pill>{graph.project.defaultArchitecture}</Pill>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">
              Root image: {graph.project.rootImageRef}
              {graph.project.namespace ? ` · ${graph.project.namespace}/${graph.project.imageName}` : ` · ${graph.project.imageName}`}
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={onArchiveProject} disabled={archiveProjectPending || graph.project.status === "archived"}>
            <Archive aria-hidden="true" />
            Archive
          </Button>
        </div>
      </section>

      <section className="grid gap-4 xl:grid-cols-[1fr_360px]">
        <div className="rounded-lg border bg-card p-5 text-card-foreground">
          <div className="mb-4 flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold">Version Graph</h3>
              <p className="mt-1 text-xs text-muted-foreground">Git-style nodes are grouped by creation order for this milestone.</p>
            </div>
            <div className="text-xs text-muted-foreground">{graph.nodes.length} nodes</div>
          </div>

          <div className="space-y-3">
            {graph.nodes.map((node, index) => (
              <button
                key={node.id}
                type="button"
                className={cn(
                  "grid w-full grid-cols-[28px_1fr] gap-3 rounded-md border bg-background p-3 text-left transition-colors hover:border-ring",
                  selectedNode?.id === node.id && "border-ring ring-2 ring-ring/20",
                )}
                onClick={() => onSelectNode(node.id)}
              >
                <div className="relative flex justify-center">
                  {index < graph.nodes.length - 1 ? <span className="absolute top-7 h-[calc(100%+14px)] w-px bg-border" aria-hidden="true" /> : null}
                  <span className="relative z-10 flex size-7 items-center justify-center rounded-full border bg-card">
                    <GitCommit className="size-4" aria-hidden="true" />
                  </span>
                </div>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium">{node.version}</span>
                    <Pill>{node.branchName}</Pill>
                    <Pill>{node.status}</Pill>
                  </div>
                  <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{node.description || "No description."}</p>
                  <p className="mt-1 text-xs text-muted-foreground">Parent: {node.parentNodeId ?? "-"}</p>
                </div>
              </button>
            ))}
          </div>
        </div>

        <aside className="space-y-4">
          <section className="rounded-lg border bg-card p-4 text-card-foreground">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold">Branches</h3>
              <GitBranch className="size-4 text-muted-foreground" aria-hidden="true" />
            </div>
            <div className="mt-3 space-y-2">
              {graph.branches.map((branch) => (
                <div key={branch.id} className="rounded-md border bg-background p-3 text-sm">
                  <div className="flex items-center justify-between gap-3">
                    <div className="font-medium">{branch.name}</div>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onArchiveBranch(branch.id)}
                      disabled={branch.status === "archived" || archiveBranchPending || branch.name === "main"}
                    >
                      Archive
                    </Button>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">Head: {branch.headNodeId ?? "-"}</p>
                </div>
              ))}
            </div>
          </section>

          <section className="rounded-lg border bg-card p-4 text-card-foreground">
            <h3 className="text-sm font-semibold">Selected Node</h3>
            {selectedNode ? (
              <div className="mt-3 space-y-3">
                <div>
                  <div className="text-sm font-medium">{selectedNode.version}</div>
                  <p className="text-xs text-muted-foreground">{selectedNode.description || "No description."}</p>
                </div>
                <pre className="max-h-64 overflow-auto rounded-md border bg-background p-3 text-xs">{selectedNode.dockerfile}</pre>
                <div className="grid grid-cols-2 gap-2">
                  <Button variant="outline" size="sm" onClick={() => onPrepareBranch(selectedNode)}>
                    <GitBranch aria-hidden="true" />
                    Branch
                  </Button>
                  <Button size="sm" onClick={() => onPrepareVersion(selectedNode, graph)}>
                    <Plus aria-hidden="true" />
                    Version
                  </Button>
                </div>
              </div>
            ) : (
              <p className="mt-2 text-sm text-muted-foreground">Select a node to view Dockerfile and create descendants.</p>
            )}
          </section>

          <section className="rounded-lg border bg-card p-4 text-card-foreground">
            <h3 className="text-sm font-semibold">Create Branch</h3>
            <div className="mt-3 space-y-3">
              <Field label="Name">
                <input className={inputClassName} value={branchForm.name} onChange={(event) => updateForm(onBranchFormChange, "name", event.target.value)} />
              </Field>
              <Field label="Start Node">
                <select className={inputClassName} value={branchForm.startNodeId} onChange={(event) => updateForm(onBranchFormChange, "startNodeId", event.target.value)}>
                  <option value="">Select node</option>
                  {graph.nodes.map((node) => (
                    <option key={node.id} value={node.id}>
                      {node.version}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Description">
                <input className={inputClassName} value={branchForm.description} onChange={(event) => updateForm(onBranchFormChange, "description", event.target.value)} />
              </Field>
              <Button className="w-full" onClick={onCreateBranch} disabled={createBranchPending || !branchForm.name || !branchForm.startNodeId}>
                {createBranchPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <GitBranch aria-hidden="true" />}
                Create Branch
              </Button>
            </div>
          </section>

          <section className="rounded-lg border bg-card p-4 text-card-foreground">
            <h3 className="text-sm font-semibold">Create Version</h3>
            <div className="mt-3 space-y-3">
              <Field label="Branch">
                <select className={inputClassName} value={nodeForm.branchId} onChange={(event) => updateForm(onNodeFormChange, "branchId", event.target.value)}>
                  <option value="">Select branch</option>
                  {activeBranches.map((branch) => (
                    <option key={branch.id} value={branch.id}>
                      {branch.name}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Parent Node">
                <select className={inputClassName} value={nodeForm.parentNodeId} onChange={(event) => updateForm(onNodeFormChange, "parentNodeId", event.target.value)}>
                  <option value="">Use branch head</option>
                  {graph.nodes.map((node) => (
                    <option key={node.id} value={node.id}>
                      {node.version}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Version">
                <input className={inputClassName} value={nodeForm.version} onChange={(event) => updateForm(onNodeFormChange, "version", event.target.value)} />
              </Field>
              <Field label="Dockerfile">
                <textarea
                  className="min-h-40 w-full rounded-md border bg-background px-3 py-2 font-mono text-xs outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
                  value={nodeForm.dockerfile}
                  onChange={(event) => updateForm(onNodeFormChange, "dockerfile", event.target.value)}
                />
              </Field>
              <Field label="Description">
                <input className={inputClassName} value={nodeForm.description} onChange={(event) => updateForm(onNodeFormChange, "description", event.target.value)} />
              </Field>
              <Button className="w-full" onClick={onCreateNode} disabled={createNodePending || !nodeForm.branchId || !nodeForm.version || !nodeForm.dockerfile}>
                {createNodePending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
                Create Version
              </Button>
            </div>
          </section>
        </aside>
      </section>
    </>
  )
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block space-y-2 text-sm font-medium">
      <span>{label}</span>
      {children}
    </label>
  )
}

function StateBlock({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="p-6 text-center">
      <h3 className="text-base font-semibold">{title}</h3>
      <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
    </div>
  )
}

function Pill({ children }: { children: ReactNode }) {
  return <span className="inline-flex items-center rounded-md bg-secondary px-1.5 py-0.5 text-xs text-secondary-foreground">{children}</span>
}

function toProjectInput(form: ProjectFormState): ProjectInput {
  return {
    name: form.name,
    imageType: form.imageType,
    imageName: form.imageName,
    namespace: form.namespace,
    rootImageRef: form.rootImageRef,
    rootImageSource: "external_image",
    defaultArchitecture: form.defaultArchitecture,
    labels: form.labels
      .split(",")
      .map((label) => label.trim())
      .filter(Boolean),
    description: form.description,
  }
}

function updateForm<T extends Record<string, unknown>>(setForm: Dispatch<SetStateAction<T>>, key: keyof T, value: string) {
  setForm((current) => ({ ...current, [key]: value }))
}

const inputClassName =
  "h-10 w-full rounded-md border bg-background px-3 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
