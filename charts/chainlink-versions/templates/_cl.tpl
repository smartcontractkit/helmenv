{{/*
Chainlink components:
Inputs:
    .Values of a particular Chainlink version
    .Release common release name
    .idx deployment index
*/}}

{{- define "chainlink.secret" }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Release.Name }}-{{ .idx }}-node-creds-secret
type: Opaque
data:
  nodepassword: VC50TEhrY213ZVBUL3AsXXNZdW50andIS0FzcmhtIzRlUnM0THVLSHd2SGVqV1lBQzJKUDRNOEhpbXdnbWJhWgo=
  apicredentials: bm90cmVhbEBmYWtlZW1haWwuY2hudHdvY2hhaW5zCg==
  node-password: VC50TEhrY213ZVBUL3AsXXNZdW50andIS0FzcmhtIzRlUnM0THVLSHd2SGVqV1lBQzJKUDRNOEhpbXdnbWJhWgo=
{{ end }}

{{- define "chainlink.configMap" }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app: {{ .Release.Name }}-{{ .idx }}-cm
    release: {{ .Release.Name }}
  name: {{ .Release.Name }}-{{ .idx }}-cm
data:
  apicredentials: |
    notreal@fakeemail.ch
    twochains
  node-password: T.tLHkcmwePT/p,]sYuntjwHKAsrhm#4eRs4LuKHwvHejWYAC2JP4M8HimwgmbaZ
---
{{ end }}

{{- define "chainlink.service" }}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Release.Name }}-{{ .idx }}-service
spec:
  ports:
    - name: node-port
      port: {{ .Values.chainlink.web_port }}
      targetPort: {{ .Values.chainlink.web_port }}
    - name: p2p-port
      port: {{ .Values.chainlink.p2p_port }}
      targetPort: {{ .Values.chainlink.p2p_port }}
  selector:
    app: {{ .Release.Name }}-{{ .idx }}-node
  type: ClusterIP
---
{{ end }}

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
  replicas: {{ .Values.replicas }}
  selector:
    matchLabels:
      app: {{ .Release.Name }}-{{ .idx }}-node
      release: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app: {{ .Release.Name }}-{{ .idx }}-node
        release: {{ .Release.Name }}
      annotations:
        prometheus.io/scrape: 'true'
    spec:
      volumes:
        - name: {{ .Release.Name }}-config-map
          configMap:
            name: {{ .Release.Name }}-{{ .idx }}-cm
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
          image: {{ .Values.chainlink.image }}:{{ .Values.chainlink.version }}
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
              {{- if kindIs "string" $value }}
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
