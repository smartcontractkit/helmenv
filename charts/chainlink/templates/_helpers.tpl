{{/*
Expand the name of the chart.
*/}}
{{- define "chainlink.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "chainlink.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "chainlink.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "chainlink.labels" -}}
helm.sh/chart: {{ include "chainlink.chart" . }}
{{ include "chainlink.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "chainlink.selectorLabels" -}}
app.kubernetes.io/name: {{ include "chainlink.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "chainlink.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "chainlink.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "chainlink.deployment" -}}
---
apiVersion: apps/v1
{{ if .Values.db.stateful }}
kind: StatefulSet
{{ else }}
kind: Deployment
{{ end }}
metadata:
  name: {{ .Release.Name }}-{{ .idx }}-node
spec:
  {{ if .Values.db.stateful }}
  serviceName: {{ .Release.Name }}-service
  podManagementPolicy: Parallel
  volumeClaimTemplates:
    - metadata:
        name: postgres
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: {{ .Values.db.capacity }}
  {{ end }}
  {{ if .Values.chainlink.versions }}
  replicas: 1
  {{ else }}
  replicas: {{ .Values.replicas }}
  {{ end }}
  selector:
    matchLabels:
      app: {{ .Release.Name }}-node
      release: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app: {{ .Release.Name }}-node
        release: {{ .Release.Name }}
      annotations:
        prometheus.io/scrape: 'true'
    spec:
      volumes:
        - name: {{ .Release.Name }}-config-map
          configMap:
            name: {{ .Release.Name }}-cm
      containers:
        - name: chainlink-db
          image: postgres:11.6
          ports:
            - name: postgres
              containerPort: 5432
          env:
            - name: POSTGRES_DB
              value: chainlink
            - name: POSTGRES_PASSWORD
              value: node
            - name: PGPASSWORD
              value: node
            - name: PGUSER
              value: postgres
          livenessProbe:
            exec:
              command:
                - pg_isready
                - -U
                - postgres
            initialDelaySeconds: 60
            periodSeconds: 60
          readinessProbe:
            exec:
              command:
                - pg_isready
                - -U
                - postgres
            initialDelaySeconds: 2
            periodSeconds: 2
          resources:
            requests:
              memory: {{ .Values.db.resources.requests.memory }}
              cpu: {{ .Values.db.resources.requests.cpu }}
            limits:
              memory: {{ .Values.db.resources.limits.memory }}
              cpu: {{ .Values.db.resources.limits.cpu }}
          {{ if .Values.db.stateful }}
          volumeMounts:
            - mountPath: /var/lib/postgresql/data
              name: postgres
              subPath: postgres-db
          {{ end }}
        - name: node
          image: {{ .Values.chainlink.image.image }}:{{ .ver }}
          imagePullPolicy: IfNotPresent
          args:
            - node
            - start
            - -d
            - -p
            - /etc/node-secrets-volume/node-password
            - -a
            - /etc/node-secrets-volume/apicredentials
            - --vrfpassword=/etc/node-secrets-volume/apicredentials
          ports:
            - name: access
              containerPort: {{ .Values.chainlink.web_port }}
            - name: p2p
              containerPort: {{ .Values.chainlink.p2p_port }}
          env:
          {{- range $key, $value := .Values.env }}
            {{- if $value }}
            - name: {{ $key | upper}}
              {{- if kindIs "string" $value}}
              value: {{ $value | quote}}
              {{- else }}
              value: {{ $value }}
              {{- end }}
            {{- end }}
          {{- end }}
          volumeMounts:
            - name: {{ .Release.Name }}-config-map
              mountPath: /etc/node-secrets-volume/
          livenessProbe:
            httpGet:
              path: /
              port: {{ .Values.chainlink.web_port }}
            initialDelaySeconds: 10
            periodSeconds: 5
          readinessProbe:
            httpGet:
              path: /
              port: {{ .Values.chainlink.web_port }}
            initialDelaySeconds: 10
            periodSeconds: 5
          resources:
            requests:
              memory: {{ .Values.chainlink.resources.requests.memory }}
              cpu: {{ .Values.chainlink.resources.requests.cpu }}
            limits:
              memory: {{ .Values.chainlink.resources.limits.memory }}
              cpu: {{ .Values.chainlink.resources.limits.cpu }}
---
{{- end }}
