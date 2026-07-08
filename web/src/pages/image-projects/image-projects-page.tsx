import { type Dispatch, type FormEvent, type ReactNode, type SetStateAction, useEffect, useMemo, useState } from "react"
import {
  Background,
  Controls,
  MarkerType,
  MiniMap,
  Position,
  ReactFlow,
  type Edge,
  type Node,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"
import Editor from "@monaco-editor/react"
import {
  Archive,
  Boxes,
  CheckCircle2,
  Code2,
  GitBranch,
  Loader2,
  Plus,
  Rocket,
  Search,
  Split,
  Wand2,
  XCircle,
} from "lucide-react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { Button } from "@/components/ui/button"
import { listBuildHosts, type BuildHost } from "@/lib/build-hosts-api"
import { createBuildTask, type BuildTask } from "@/lib/build-tasks-api"
import {
  generateDockerfile,
  validateDockerfile,
  type DockerfileValidationResult,
} from "@/lib/dockerfile-api"
import {
  archiveBranch,
  archiveImageProject,
  createBranch,
  createImageProject,
  createVersionNode,
  diffVersionNodes,
  getVersionGraph,
  listImageProjects,
  updateVersionNode,
  type ImageProject,
  type ImageType,
  type ProjectInput,
  type VersionGraph,
  type VersionNode,
} from "@/lib/image-projects-api"
import { listRegistries, type Registry } from "@/lib/registries-api"
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

type GeneratorFormState = {
  baseImage: string
  packages: string
  workdir: string
  expose: string
  cmd: string
  environment: string
}

type BuildFormState = {
  registryId: string
  requestedHostId: string
  architecture: string
  imageName: string
  imageTag: string
  buildArgs: string
}

type FlowData = {
  label: ReactNode
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

const defaultGeneratorForm: GeneratorFormState = {
  baseImage: "ubuntu:24.04",
  packages: "curl,ca-certificates",
  workdir: "/app",
  expose: "8080",
  cmd: "./server",
  environment: "APP_ENV=production",
}

const defaultBuildForm: BuildFormState = {
  registryId: "",
  requestedHostId: "",
  architecture: "",
  imageName: "",
  imageTag: "",
  buildArgs: "",
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
  const [generatorForm, setGeneratorForm] = useState(defaultGeneratorForm)
  const [buildForm, setBuildForm] = useState(defaultBuildForm)
  const [dockerfileDraft, setDockerfileDraft] = useState("")
  const [validationResult, setValidationResult] = useState<DockerfileValidationResult | null>(null)
  const [compareNodeId, setCompareNodeId] = useState("")
  const [diffText, setDiffText] = useState("")
  const [buildTaskResult, setBuildTaskResult] = useState<BuildTask | null>(null)
  const [buildTaskError, setBuildTaskError] = useState("")
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

  const registriesQuery = useQuery({
    queryKey: ["registries"],
    queryFn: listRegistries,
  })

  const buildHostsQuery = useQuery({
    queryKey: ["build-hosts"],
    queryFn: listBuildHosts,
  })

  const selectedNode = useMemo(() => {
    const nodes = graphQuery.data?.nodes ?? []
    return nodes.find((node) => node.id === selectedNodeId) ?? nodes[0] ?? null
  }, [graphQuery.data?.nodes, selectedNodeId])

  useEffect(() => {
    setDockerfileDraft(selectedNode?.dockerfile ?? "")
    setValidationResult(null)
    setCompareNodeId("")
    setDiffText("")
  }, [selectedNode?.id, selectedNode?.dockerfile])

  useEffect(() => {
    setBuildForm({
      ...defaultBuildForm,
      architecture: selectedProject?.defaultArchitecture ?? "",
      imageTag: selectedNode?.version ?? "",
    })
    setBuildTaskResult(null)
    setBuildTaskError("")
  }, [selectedNode?.id, selectedNode?.version, selectedProject?.defaultArchitecture])

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

  const saveNodeMutation = useMutation({
    mutationFn: ({ projectId, node, dockerfile }: { projectId: string; node: VersionNode; dockerfile: string }) =>
      updateVersionNode(projectId, node.id, {
        branchId: node.branchId,
        parentNodeId: node.parentNodeId ?? "",
        version: node.version,
        dockerfile,
        description: node.description ?? "",
        status: node.status,
      }),
    onSuccess: (node) => {
      setSelectedNodeId(node.id)
      return queryClient.invalidateQueries({ queryKey: ["image-projects", selectedProject?.id, "graph"] })
    },
  })

  const validateMutation = useMutation({
    mutationFn: validateDockerfile,
    onSuccess: setValidationResult,
  })

  const generateMutation = useMutation({
    mutationFn: () => generateDockerfile(toGenerateInput(generatorForm)),
    onSuccess: (dockerfile) => {
      setNodeForm((current) => ({ ...current, dockerfile }))
    },
  })

  const diffMutation = useMutation({
    mutationFn: ({ projectId, leftNodeId, rightNodeId }: { projectId: string; leftNodeId: string; rightNodeId: string }) =>
      diffVersionNodes(projectId, leftNodeId, rightNodeId),
    onSuccess: (diff) => setDiffText(diff.unifiedDiff),
  })

  const createBuildTaskMutation = useMutation({
    mutationFn: ({ projectId, nodeId, input }: { projectId: string; nodeId: string; input: BuildFormState }) =>
      createBuildTask({
        projectId,
        versionNodeId: nodeId,
        registryId: input.registryId,
        requestedHostId: input.requestedHostId,
        architecture: input.architecture,
        imageName: input.imageName,
        imageTag: input.imageTag,
        buildArgs: parseKeyValues(input.buildArgs),
      }),
    onSuccess: (task) => {
      setBuildTaskResult(task)
      setBuildTaskError("")
      return queryClient.invalidateQueries({ queryKey: ["build-tasks"] })
    },
    onError: (error) => {
      setBuildTaskResult(null)
      setBuildTaskError(error instanceof Error ? error.message : "创建构建任务失败。")
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
    setBranchForm({ name: `${node.version}-branch`, startNodeId: node.id, description: "" })
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
        <ProjectCatalog
          projectsPending={projectsQuery.isPending}
          projectsError={projectsQuery.isError}
          filteredProjects={filteredProjects}
          selectedProjectId={selectedProject?.id ?? null}
          keyword={keyword}
          showProjectForm={showProjectForm}
          projectForm={projectForm}
          projectError={projectError}
          createProjectPending={createProjectMutation.isPending}
          onKeywordChange={setKeyword}
          onShowProjectFormChange={setShowProjectForm}
          onProjectFormChange={setProjectForm}
          onCreateProject={handleCreateProject}
          onSelectProject={(project) => {
            setSelectedProjectId(project.id)
            setSelectedNodeId(project.latestVersionNodeId ?? null)
          }}
        />
      </aside>

      <main className="space-y-4">
        {!selectedProject ? (
          <EmptyWorkspace />
        ) : graphQuery.isPending ? (
          <LoadingWorkspace />
        ) : graphQuery.isError ? (
          <ErrorWorkspace />
        ) : (
          <GraphWorkspace
            graph={graphQuery.data}
            selectedNode={selectedNode}
            branchForm={branchForm}
            nodeForm={nodeForm}
            generatorForm={generatorForm}
            buildForm={buildForm}
            registries={registriesQuery.data ?? []}
            buildHosts={buildHostsQuery.data ?? []}
            dockerfileDraft={dockerfileDraft}
            validationResult={validationResult}
            compareNodeId={compareNodeId}
            diffText={diffText}
            buildTaskResult={buildTaskResult}
            buildTaskError={buildTaskError}
            createBranchPending={createBranchMutation.isPending}
            createNodePending={createNodeMutation.isPending}
            archiveProjectPending={archiveProjectMutation.isPending}
            archiveBranchPending={archiveBranchMutation.isPending}
            saveNodePending={saveNodeMutation.isPending}
            validatePending={validateMutation.isPending}
            generatePending={generateMutation.isPending}
            diffPending={diffMutation.isPending}
            createBuildTaskPending={createBuildTaskMutation.isPending}
            onSelectNode={setSelectedNodeId}
            onPrepareBranch={prepareBranchFromNode}
            onPrepareVersion={prepareVersionFromNode}
            onBranchFormChange={setBranchForm}
            onNodeFormChange={setNodeForm}
            onGeneratorFormChange={setGeneratorForm}
            onBuildFormChange={setBuildForm}
            onDockerfileDraftChange={setDockerfileDraft}
            onCompareNodeChange={setCompareNodeId}
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
            onSaveDockerfile={(node) => {
              if (selectedProject) {
                saveNodeMutation.mutate({ projectId: selectedProject.id, node, dockerfile: dockerfileDraft })
              }
            }}
            onValidateDockerfile={() => validateMutation.mutate(dockerfileDraft)}
            onGenerateDockerfile={() => generateMutation.mutate()}
            onDiff={() => {
              if (selectedProject && selectedNode && compareNodeId) {
                diffMutation.mutate({ projectId: selectedProject.id, leftNodeId: compareNodeId, rightNodeId: selectedNode.id })
              }
            }}
            onCreateBuildTask={() => {
              if (selectedProject && selectedNode) {
                createBuildTaskMutation.mutate({ projectId: selectedProject.id, nodeId: selectedNode.id, input: buildForm })
              }
            }}
          />
        )}
      </main>
    </div>
  )
}

function ProjectCatalog({
  projectsPending,
  projectsError,
  filteredProjects,
  selectedProjectId,
  keyword,
  showProjectForm,
  projectForm,
  projectError,
  createProjectPending,
  onKeywordChange,
  onShowProjectFormChange,
  onProjectFormChange,
  onCreateProject,
  onSelectProject,
}: {
  projectsPending: boolean
  projectsError: boolean
  filteredProjects: ImageProject[]
  selectedProjectId: string | null
  keyword: string
  showProjectForm: boolean
  projectForm: ProjectFormState
  projectError: string
  createProjectPending: boolean
  onKeywordChange: (value: string) => void
  onShowProjectFormChange: Dispatch<SetStateAction<boolean>>
  onProjectFormChange: Dispatch<SetStateAction<ProjectFormState>>
  onCreateProject: (event: FormEvent<HTMLFormElement>) => void
  onSelectProject: (project: ImageProject) => void
}) {
  return (
    <>
      <section className="rounded-lg border bg-card p-4 text-card-foreground">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h2 className="text-base font-semibold">Image Projects</h2>
            <p className="mt-1 text-sm text-muted-foreground">Select a project before viewing its version line.</p>
          </div>
          <Button size="sm" onClick={() => onShowProjectFormChange((value) => !value)}>
            <Plus aria-hidden="true" />
            Create
          </Button>
        </div>

        <label className="mt-4 flex h-10 items-center gap-2 rounded-md border bg-background px-3 text-sm">
          <Search className="size-4 text-muted-foreground" aria-hidden="true" />
          <input className="min-w-0 flex-1 bg-transparent outline-none" placeholder="Search projects" value={keyword} onChange={(event) => onKeywordChange(event.target.value)} />
        </label>

        {showProjectForm ? (
          <form className="mt-4 space-y-3 rounded-md border bg-background p-3" onSubmit={onCreateProject}>
            <Field label="Project Name">
              <input className={inputClassName} value={projectForm.name} onChange={(event) => updateForm(onProjectFormChange, "name", event.target.value)} required />
            </Field>
            <Field label="Image Type">
              <select className={inputClassName} value={projectForm.imageType} onChange={(event) => updateForm(onProjectFormChange, "imageType", event.target.value)}>
                <option value="java">Java</option>
                <option value="python">Python</option>
                <option value="nodejs">Node.js</option>
                <option value="mysql">MySQL</option>
                <option value="base_os">Base OS</option>
                <option value="other">Other</option>
              </select>
            </Field>
            <Field label="Image Name">
              <input className={inputClassName} value={projectForm.imageName} onChange={(event) => updateForm(onProjectFormChange, "imageName", event.target.value)} required />
            </Field>
            <Field label="Namespace">
              <input className={inputClassName} value={projectForm.namespace} onChange={(event) => updateForm(onProjectFormChange, "namespace", event.target.value)} />
            </Field>
            <Field label="Root Image">
              <input className={inputClassName} value={projectForm.rootImageRef} onChange={(event) => updateForm(onProjectFormChange, "rootImageRef", event.target.value)} required />
            </Field>
            <Field label="Default Architecture">
              <input className={inputClassName} value={projectForm.defaultArchitecture} onChange={(event) => updateForm(onProjectFormChange, "defaultArchitecture", event.target.value)} required />
            </Field>
            <Field label="Labels">
              <input className={inputClassName} value={projectForm.labels} onChange={(event) => updateForm(onProjectFormChange, "labels", event.target.value)} />
            </Field>
            <Field label="Description">
              <textarea className={textareaClassName} value={projectForm.description} onChange={(event) => updateForm(onProjectFormChange, "description", event.target.value)} />
            </Field>
            {projectError ? <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{projectError}</div> : null}
            <Button className="w-full" type="submit" disabled={createProjectPending}>
              {createProjectPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Plus aria-hidden="true" />}
              Create Project
            </Button>
          </form>
        ) : null}
      </section>

      <section className="rounded-lg border bg-card text-card-foreground">
        {projectsPending ? (
          <StateBlock title="Loading projects" detail="Fetching image project catalog." />
        ) : projectsError ? (
          <StateBlock title="Failed to load projects" detail="Please retry after checking the backend service." />
        ) : filteredProjects.length === 0 ? (
          <StateBlock title="No projects" detail="Create a Java, Python, Node.js, MySQL, or base image project." />
        ) : (
          <div className="divide-y">
            {filteredProjects.map((project) => (
              <button
                key={project.id}
                type="button"
                className={cn("block w-full px-4 py-3 text-left transition-colors hover:bg-accent", selectedProjectId === project.id && "bg-accent")}
                onClick={() => onSelectProject(project)}
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium">{project.name}</div>
                    <div className="mt-1 truncate text-xs text-muted-foreground">
                      {project.namespace ? `${project.namespace}/` : ""}
                      {project.imageName}
                    </div>
                  </div>
                  <Pill>{project.imageType}</Pill>
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
    </>
  )
}

function GraphWorkspace({
  graph,
  selectedNode,
  branchForm,
  nodeForm,
  generatorForm,
  buildForm,
  registries,
  buildHosts,
  dockerfileDraft,
  validationResult,
  compareNodeId,
  diffText,
  buildTaskResult,
  buildTaskError,
  createBranchPending,
  createNodePending,
  archiveProjectPending,
  archiveBranchPending,
  saveNodePending,
  validatePending,
  generatePending,
  diffPending,
  createBuildTaskPending,
  onSelectNode,
  onPrepareBranch,
  onPrepareVersion,
  onBranchFormChange,
  onNodeFormChange,
  onGeneratorFormChange,
  onBuildFormChange,
  onDockerfileDraftChange,
  onCompareNodeChange,
  onCreateBranch,
  onCreateNode,
  onArchiveProject,
  onArchiveBranch,
  onSaveDockerfile,
  onValidateDockerfile,
  onGenerateDockerfile,
  onDiff,
  onCreateBuildTask,
}: {
  graph: VersionGraph
  selectedNode: VersionNode | null
  branchForm: BranchFormState
  nodeForm: NodeFormState
  generatorForm: GeneratorFormState
  buildForm: BuildFormState
  registries: Registry[]
  buildHosts: BuildHost[]
  dockerfileDraft: string
  validationResult: DockerfileValidationResult | null
  compareNodeId: string
  diffText: string
  buildTaskResult: BuildTask | null
  buildTaskError: string
  createBranchPending: boolean
  createNodePending: boolean
  archiveProjectPending: boolean
  archiveBranchPending: boolean
  saveNodePending: boolean
  validatePending: boolean
  generatePending: boolean
  diffPending: boolean
  createBuildTaskPending: boolean
  onSelectNode: (nodeId: string) => void
  onPrepareBranch: (node: VersionNode) => void
  onPrepareVersion: (node: VersionNode, graph: VersionGraph) => void
  onBranchFormChange: Dispatch<SetStateAction<BranchFormState>>
  onNodeFormChange: Dispatch<SetStateAction<NodeFormState>>
  onGeneratorFormChange: Dispatch<SetStateAction<GeneratorFormState>>
  onBuildFormChange: Dispatch<SetStateAction<BuildFormState>>
  onDockerfileDraftChange: (value: string) => void
  onCompareNodeChange: (value: string) => void
  onCreateBranch: () => void
  onCreateNode: () => void
  onArchiveProject: () => void
  onArchiveBranch: (branchId: string) => void
  onSaveDockerfile: (node: VersionNode) => void
  onValidateDockerfile: () => void
  onGenerateDockerfile: () => void
  onDiff: () => void
  onCreateBuildTask: () => void
}) {
  const activeBranches = graph.branches.filter((branch) => branch.status === "active")
  const flow = useMemo(() => toFlow(graph, selectedNode?.id), [graph, selectedNode?.id])

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

      <section className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_420px]">
        <div className="rounded-lg border bg-card p-4 text-card-foreground">
          <div className="mb-3 flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold">Version Graph</h3>
              <p className="mt-1 text-xs text-muted-foreground">Zoom, pan, drag nodes, and click a node to edit its Dockerfile.</p>
            </div>
            <div className="text-xs text-muted-foreground">{graph.nodes.length} nodes</div>
          </div>

          <div className="h-[560px] overflow-hidden rounded-md border bg-background">
            <ReactFlow
              nodes={flow.nodes}
              edges={flow.edges}
              fitView
              minZoom={0.25}
              maxZoom={1.8}
              onNodeClick={(_, node) => onSelectNode(node.id)}
              nodesDraggable
              panOnScroll
            >
              <Background gap={18} size={1} />
              <MiniMap pannable zoomable />
              <Controls />
            </ReactFlow>
          </div>
        </div>

        <aside className="space-y-4">
          <BranchesPanel
            graph={graph}
            archiveBranchPending={archiveBranchPending}
            onArchiveBranch={onArchiveBranch}
          />
          <NodeDetailPanel
            graph={graph}
            selectedNode={selectedNode}
            dockerfileDraft={dockerfileDraft}
            validationResult={validationResult}
            compareNodeId={compareNodeId}
            diffText={diffText}
            buildForm={buildForm}
            registries={registries}
            buildHosts={buildHosts}
            buildTaskResult={buildTaskResult}
            buildTaskError={buildTaskError}
            saveNodePending={saveNodePending}
            validatePending={validatePending}
            diffPending={diffPending}
            createBuildTaskPending={createBuildTaskPending}
            onPrepareBranch={onPrepareBranch}
            onPrepareVersion={onPrepareVersion}
            onBuildFormChange={onBuildFormChange}
            onDockerfileDraftChange={onDockerfileDraftChange}
            onCompareNodeChange={onCompareNodeChange}
            onSaveDockerfile={onSaveDockerfile}
            onValidateDockerfile={onValidateDockerfile}
            onDiff={onDiff}
            onCreateBuildTask={onCreateBuildTask}
          />
          <CreateBranchPanel
            graph={graph}
            branchForm={branchForm}
            createBranchPending={createBranchPending}
            onBranchFormChange={onBranchFormChange}
            onCreateBranch={onCreateBranch}
          />
          <CreateVersionPanel
            graph={graph}
            activeBranches={activeBranches}
            nodeForm={nodeForm}
            generatorForm={generatorForm}
            createNodePending={createNodePending}
            generatePending={generatePending}
            onNodeFormChange={onNodeFormChange}
            onGeneratorFormChange={onGeneratorFormChange}
            onGenerateDockerfile={onGenerateDockerfile}
            onCreateNode={onCreateNode}
          />
        </aside>
      </section>
    </>
  )
}

function BranchesPanel({
  graph,
  archiveBranchPending,
  onArchiveBranch,
}: {
  graph: VersionGraph
  archiveBranchPending: boolean
  onArchiveBranch: (branchId: string) => void
}) {
  return (
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
              <Button variant="ghost" size="sm" onClick={() => onArchiveBranch(branch.id)} disabled={branch.status === "archived" || archiveBranchPending || branch.name === "main"}>
                Archive
              </Button>
            </div>
            <p className="mt-1 text-xs text-muted-foreground">Head: {branch.headNodeId ?? "-"}</p>
          </div>
        ))}
      </div>
    </section>
  )
}

function NodeDetailPanel({
  graph,
  selectedNode,
  dockerfileDraft,
  validationResult,
  compareNodeId,
  diffText,
  buildForm,
  registries,
  buildHosts,
  buildTaskResult,
  buildTaskError,
  saveNodePending,
  validatePending,
  diffPending,
  createBuildTaskPending,
  onPrepareBranch,
  onPrepareVersion,
  onBuildFormChange,
  onDockerfileDraftChange,
  onCompareNodeChange,
  onSaveDockerfile,
  onValidateDockerfile,
  onDiff,
  onCreateBuildTask,
}: {
  graph: VersionGraph
  selectedNode: VersionNode | null
  dockerfileDraft: string
  validationResult: DockerfileValidationResult | null
  compareNodeId: string
  diffText: string
  buildForm: BuildFormState
  registries: Registry[]
  buildHosts: BuildHost[]
  buildTaskResult: BuildTask | null
  buildTaskError: string
  saveNodePending: boolean
  validatePending: boolean
  diffPending: boolean
  createBuildTaskPending: boolean
  onPrepareBranch: (node: VersionNode) => void
  onPrepareVersion: (node: VersionNode, graph: VersionGraph) => void
  onBuildFormChange: Dispatch<SetStateAction<BuildFormState>>
  onDockerfileDraftChange: (value: string) => void
  onCompareNodeChange: (value: string) => void
  onSaveDockerfile: (node: VersionNode) => void
  onValidateDockerfile: () => void
  onDiff: () => void
  onCreateBuildTask: () => void
}) {
  return (
    <section className="rounded-lg border bg-card p-4 text-card-foreground">
      <h3 className="text-sm font-semibold">Node Detail</h3>
      {selectedNode ? (
        <div className="mt-3 space-y-3">
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">{selectedNode.version}</span>
              <Pill>{selectedNode.branchName}</Pill>
              <Pill>{selectedNode.status}</Pill>
            </div>
            <p className="mt-1 text-xs text-muted-foreground">{selectedNode.description || "No description."}</p>
          </div>

          <div className="h-72 overflow-hidden rounded-md border">
            <Editor
              height="100%"
              language="dockerfile"
              theme="vs-light"
              value={dockerfileDraft}
              options={{ minimap: { enabled: false }, fontSize: 12, wordWrap: "on", scrollBeyondLastLine: false }}
              onChange={(value) => onDockerfileDraftChange(value ?? "")}
            />
          </div>

          {validationResult ? <ValidationSummary result={validationResult} /> : null}

          <div className="grid grid-cols-2 gap-2">
            <Button variant="outline" size="sm" onClick={() => onPrepareBranch(selectedNode)}>
              <GitBranch aria-hidden="true" />
              Branch
            </Button>
            <Button variant="outline" size="sm" onClick={() => onPrepareVersion(selectedNode, graph)}>
              <Plus aria-hidden="true" />
              Version
            </Button>
            <Button variant="outline" size="sm" onClick={onValidateDockerfile} disabled={validatePending}>
              {validatePending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Code2 aria-hidden="true" />}
              Validate
            </Button>
            <Button size="sm" onClick={() => onSaveDockerfile(selectedNode)} disabled={saveNodePending}>
              {saveNodePending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <CheckCircle2 aria-hidden="true" />}
              Save
            </Button>
          </div>

          <div className="space-y-2 rounded-md border bg-background p-3">
            <Field label="Compare With">
              <select className={inputClassName} value={compareNodeId} onChange={(event) => onCompareNodeChange(event.target.value)}>
                <option value="">Select node</option>
                {graph.nodes
                  .filter((node) => node.id !== selectedNode.id)
                  .map((node) => (
                    <option key={node.id} value={node.id}>
                      {node.version}
                    </option>
                  ))}
              </select>
            </Field>
            <Button className="w-full" variant="outline" size="sm" onClick={onDiff} disabled={!compareNodeId || diffPending}>
              {diffPending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Split aria-hidden="true" />}
              Diff Dockerfile
            </Button>
            {diffText ? <pre className="max-h-56 overflow-auto rounded-md border bg-card p-3 text-xs">{diffText}</pre> : null}
          </div>

          <CreateBuildTaskPanel
            form={buildForm}
            registries={registries}
            buildHosts={buildHosts}
            result={buildTaskResult}
            error={buildTaskError}
            pending={createBuildTaskPending}
            onFormChange={onBuildFormChange}
            onCreate={onCreateBuildTask}
          />
        </div>
      ) : (
        <p className="mt-2 text-sm text-muted-foreground">Select a node to edit Dockerfile and create descendants.</p>
      )}
    </section>
  )
}

function CreateBuildTaskPanel({
  form,
  registries,
  buildHosts,
  result,
  error,
  pending,
  onFormChange,
  onCreate,
}: {
  form: BuildFormState
  registries: Registry[]
  buildHosts: BuildHost[]
  result: BuildTask | null
  error: string
  pending: boolean
  onFormChange: Dispatch<SetStateAction<BuildFormState>>
  onCreate: () => void
}) {
  const pushRegistries = registries.filter((registry) => registry.allowPush && registry.status !== "disabled")
  const schedulableHosts = buildHosts.filter((host) => host.status !== "disabled")

  return (
    <div className="space-y-3 rounded-md border bg-background p-3">
      <div className="flex items-center gap-2 text-sm font-medium">
        <Rocket className="size-4" aria-hidden="true" />
        Build Task
      </div>
      <Field label="Registry">
        <select className={inputClassName} value={form.registryId} onChange={(event) => updateForm(onFormChange, "registryId", event.target.value)}>
          <option value="">Use project/default push registry</option>
          {pushRegistries.map((registry) => (
            <option key={registry.id} value={registry.id}>
              {registry.name}
            </option>
          ))}
        </select>
      </Field>
      <Field label="Build Host">
        <select className={inputClassName} value={form.requestedHostId} onChange={(event) => updateForm(onFormChange, "requestedHostId", event.target.value)}>
          <option value="">Auto schedule</option>
          {schedulableHosts.map((host) => (
            <option key={host.id} value={host.id}>
              {host.name}
              {host.architecture ? ` (${host.architecture})` : ""}
            </option>
          ))}
        </select>
      </Field>
      <div className="grid gap-2 sm:grid-cols-2">
        <Field label="Architecture">
          <input className={inputClassName} value={form.architecture} onChange={(event) => updateForm(onFormChange, "architecture", event.target.value)} />
        </Field>
        <Field label="Image Tag">
          <input className={inputClassName} value={form.imageTag} onChange={(event) => updateForm(onFormChange, "imageTag", event.target.value)} />
        </Field>
      </div>
      <Field label="Image Name">
        <input className={inputClassName} value={form.imageName} onChange={(event) => updateForm(onFormChange, "imageName", event.target.value)} />
      </Field>
      <Field label="Build Args">
        <input className={inputClassName} value={form.buildArgs} onChange={(event) => updateForm(onFormChange, "buildArgs", event.target.value)} />
      </Field>
      {error ? <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">{error}</div> : null}
      {result ? <div className="rounded-md border border-sky-200 bg-sky-50 px-3 py-2 text-sm text-sky-700">Queued: {result.imageRef}</div> : null}
      <Button className="w-full" onClick={onCreate} disabled={pending}>
        {pending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Rocket aria-hidden="true" />}
        Create Build Task
      </Button>
    </div>
  )
}

function CreateBranchPanel({
  graph,
  branchForm,
  createBranchPending,
  onBranchFormChange,
  onCreateBranch,
}: {
  graph: VersionGraph
  branchForm: BranchFormState
  createBranchPending: boolean
  onBranchFormChange: Dispatch<SetStateAction<BranchFormState>>
  onCreateBranch: () => void
}) {
  return (
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
  )
}

function CreateVersionPanel({
  graph,
  activeBranches,
  nodeForm,
  generatorForm,
  createNodePending,
  generatePending,
  onNodeFormChange,
  onGeneratorFormChange,
  onGenerateDockerfile,
  onCreateNode,
}: {
  graph: VersionGraph
  activeBranches: VersionGraph["branches"]
  nodeForm: NodeFormState
  generatorForm: GeneratorFormState
  createNodePending: boolean
  generatePending: boolean
  onNodeFormChange: Dispatch<SetStateAction<NodeFormState>>
  onGeneratorFormChange: Dispatch<SetStateAction<GeneratorFormState>>
  onGenerateDockerfile: () => void
  onCreateNode: () => void
}) {
  return (
    <section className="rounded-lg border bg-card p-4 text-card-foreground">
      <h3 className="text-sm font-semibold">Create Version</h3>
      <div className="mt-3 space-y-3">
        <div className="rounded-md border bg-background p-3">
          <div className="mb-3 flex items-center gap-2 text-sm font-medium">
            <Wand2 className="size-4" aria-hidden="true" />
            Form Generator
          </div>
          <div className="grid gap-2">
            <Field label="Base Image">
              <input className={inputClassName} value={generatorForm.baseImage} onChange={(event) => updateForm(onGeneratorFormChange, "baseImage", event.target.value)} />
            </Field>
            <Field label="Packages">
              <input className={inputClassName} value={generatorForm.packages} onChange={(event) => updateForm(onGeneratorFormChange, "packages", event.target.value)} />
            </Field>
            <Field label="Workdir">
              <input className={inputClassName} value={generatorForm.workdir} onChange={(event) => updateForm(onGeneratorFormChange, "workdir", event.target.value)} />
            </Field>
            <Field label="Expose">
              <input className={inputClassName} value={generatorForm.expose} onChange={(event) => updateForm(onGeneratorFormChange, "expose", event.target.value)} />
            </Field>
            <Field label="CMD">
              <input className={inputClassName} value={generatorForm.cmd} onChange={(event) => updateForm(onGeneratorFormChange, "cmd", event.target.value)} />
            </Field>
            <Field label="Environment">
              <input className={inputClassName} value={generatorForm.environment} onChange={(event) => updateForm(onGeneratorFormChange, "environment", event.target.value)} />
            </Field>
            <Button className="w-full" variant="outline" size="sm" onClick={onGenerateDockerfile} disabled={generatePending}>
              {generatePending ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Wand2 aria-hidden="true" />}
              Generate Dockerfile
            </Button>
          </div>
        </div>

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
          <textarea className="min-h-40 w-full rounded-md border bg-background px-3 py-2 font-mono text-xs outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20" value={nodeForm.dockerfile} onChange={(event) => updateForm(onNodeFormChange, "dockerfile", event.target.value)} />
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
  )
}

function ValidationSummary({ result }: { result: DockerfileValidationResult }) {
  return (
    <div className={cn("rounded-md border px-3 py-2 text-sm", result.valid ? "border-emerald-200 bg-emerald-50 text-emerald-700" : "border-red-200 bg-red-50 text-red-700")}>
      <div className="flex items-center gap-2 font-medium">
        {result.valid ? <CheckCircle2 className="size-4" aria-hidden="true" /> : <XCircle className="size-4" aria-hidden="true" />}
        {result.valid ? "Dockerfile is valid." : "Dockerfile has errors."}
      </div>
      {result.errors.length > 0 ? <ul className="mt-2 list-disc pl-5">{result.errors.map((item) => <li key={item}>{item}</li>)}</ul> : null}
      {result.warnings.length > 0 ? <ul className="mt-2 list-disc pl-5 text-amber-700">{result.warnings.map((item) => <li key={item}>{item}</li>)}</ul> : null}
    </div>
  )
}

function EmptyWorkspace() {
  return (
    <section className="rounded-lg border bg-card p-8 text-center text-card-foreground">
      <Boxes className="mx-auto size-8 text-muted-foreground" aria-hidden="true" />
      <h2 className="mt-3 text-base font-semibold">No Project Selected</h2>
      <p className="mt-1 text-sm text-muted-foreground">Create or select a project to view its version branches.</p>
    </section>
  )
}

function LoadingWorkspace() {
  return (
    <section className="rounded-lg border bg-card p-8 text-center text-card-foreground">
      <Loader2 className="mx-auto size-6 animate-spin text-muted-foreground" aria-hidden="true" />
      <p className="mt-3 text-sm text-muted-foreground">Loading version graph.</p>
    </section>
  )
}

function ErrorWorkspace() {
  return (
    <section className="rounded-lg border bg-card p-8 text-center text-card-foreground">
      <h2 className="text-base font-semibold">Failed to Load Graph</h2>
      <p className="mt-1 text-sm text-muted-foreground">Please retry after checking the backend service.</p>
    </section>
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

function VersionNodeLabel({ node, selected }: { node: VersionNode; selected: boolean }) {
  return (
    <div className={cn("min-w-48 rounded-md border bg-card px-3 py-2 text-left shadow-sm", selected && "border-ring ring-2 ring-ring/20")}>
      <div className="flex items-center gap-2">
        <span className="size-2 rounded-full bg-zinc-400" aria-hidden="true" />
        <span className="truncate text-sm font-semibold">{node.version}</span>
      </div>
      <div className="mt-1 flex flex-wrap gap-1">
        <Pill>{node.branchName}</Pill>
        <Pill>{node.status}</Pill>
      </div>
      <p className="mt-1 max-w-48 truncate text-xs text-muted-foreground">{node.description || "No description."}</p>
    </div>
  )
}

function toFlow(graph: VersionGraph, selectedNodeId?: string): { nodes: Node<FlowData>[]; edges: Edge[] } {
  const branchOrder = new Map(graph.branches.map((branch, index) => [branch.id, index]))
  const nodeOrder = new Map(graph.nodes.map((node, index) => [node.id, index]))
  const depthCache = new Map<string, number>()

  function depth(node: VersionNode): number {
    if (depthCache.has(node.id)) {
      return depthCache.get(node.id)!
    }
    if (!node.parentNodeId) {
      depthCache.set(node.id, 0)
      return 0
    }
    const parent = graph.nodes.find((item) => item.id === node.parentNodeId)
    const value = parent ? depth(parent) + 1 : 0
    depthCache.set(node.id, value)
    return value
  }

  const nodes: Node<FlowData>[] = graph.nodes.map((node) => {
    const row = branchOrder.get(node.branchId) ?? 0
    const fallbackOrder = nodeOrder.get(node.id) ?? 0
    return {
      id: node.id,
      type: "default",
      position: { x: depth(node) * 300 + 40, y: row * 150 + fallbackOrder * 18 + 50 },
      data: { label: <VersionNodeLabel node={node} selected={selectedNodeId === node.id} /> },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
    }
  })

  const edges: Edge[] = graph.edges.map((edge) => ({
    id: edge.id,
    source: edge.source,
    target: edge.target,
    type: "smoothstep",
    label: edge.targetLabel,
    markerEnd: { type: MarkerType.ArrowClosed },
  }))

  return { nodes, edges }
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
    labels: parseList(form.labels),
    description: form.description,
  }
}

function toGenerateInput(form: GeneratorFormState) {
  return {
    baseImage: form.baseImage,
    environment: parseKeyValues(form.environment),
    workdir: form.workdir,
    packages: parseList(form.packages),
    copy: [],
    expose: parseList(form.expose)
      .map((value) => Number(value))
      .filter((value) => Number.isFinite(value) && value > 0),
    cmd: parseList(form.cmd),
    entrypoint: [],
    args: {},
    labels: {},
  }
}

function parseList(value: string) {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseKeyValues(value: string) {
  const result: Record<string, string> = {}
  for (const item of parseList(value)) {
    const [key, ...rest] = item.split("=")
    if (key && rest.length > 0) {
      result[key.trim()] = rest.join("=").trim()
    }
  }
  return result
}

function updateForm<T extends Record<string, unknown>>(setForm: Dispatch<SetStateAction<T>>, key: keyof T, value: string) {
  setForm((current) => ({ ...current, [key]: value }))
}

const inputClassName =
  "h-10 w-full rounded-md border bg-background px-3 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"

const textareaClassName =
  "min-h-20 w-full rounded-md border bg-background px-3 py-2 text-sm outline-none transition-colors focus:border-ring focus:ring-2 focus:ring-ring/20"
