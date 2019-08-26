import {env} from "../../env/parse.js";
import {obj} from "../../copy/object.js";

const BUILD_TYPE_REVISION = "BUILD_TYPE_REVISION"
const BUILD_TYPE_IMAGE = "BUILD_TYPE_IMAGE"

Vue.component('deploy-modal', {
    template: '#deploy-modal-template',
    
    props: ['instance', 'app', 'source'],

    data: function() {
        let data = {
            versions: {
                data: {},
                isLoading: false,
            },
            
            selectedSource: "",
            selectedVersion: "",
            baseVersion: "",

            override: {
                env: false
            },

            environment: "",
            secrets: "",

            deploying: false,
            deployAuthorized: (this.instance.deployCode.length == 0),

            commits: [],
            showChanges: false,
            loadingChanges: false,

            error: "",
        }

        if (this.instance.containers.length > 0) {
            data.environment = this.formatEnv(this.instance.containers[0].environment)
            data.secrets = this.formatEnv(this.instance.containers[0].secrets)
        }

        return data
    },

    watch: {
        selectedVersion: function(key) {
            let build = this.versions.data[key];

            this.baseVersion = build.version

            this.getCommits(build.version);
        },
        selectedSource: function (instance) {
            let that = this 

            this.versions.isLoading = true
            this.loadVersions(instance)
            .then(function(versions) {
                if (versions.message) {
                    that.error = versions.message;
                } 

                for (let i in that.app.instances) {
                    if (that.selectedSource == that.app.instances[i].name) {
                        versions[that.app.instances[i].formattedVersion] = {
                            type: BUILD_TYPE_REVISION,
                            revision: that.app.instances[i].revision,
                            version: that.app.instances[i].version
                        }

                        that.selectedVersion = that.app.instances[i].formattedVersion;
                    }
                }

                that.versions.data = {}

                Object.keys(versions).map(function(key, index) {
                    let build = versions[key];

                    if (build.type === BUILD_TYPE_IMAGE) {
                        build.revision = that.instance.revision;
                    }

                    build.display = that.formatVersion(key, build.revision, build.type)
                    
                    that.versions.data[key] = build;
                });

                that.versions.isLoading = false
            })
        }
    },

    mounted: function () { 
        let registry = (this.source)
            ? this.source
            : (this.instance.task.registry && this.instance.task.registry.length > 0) 
                ? this.instance.task.registry
                : this.instance.name

        for (let i in this.app.instances) {
            if (this.app.instances[i].name == registry) {
                this.selectedSource = this.app.instances[i].name
                this.selectedVersion = this.app.instances[i].formattedVersion;

                this.versions.data[this.selectedVersion] = {
                    revision: this.app.instances[i].revision,
                    version: this.app.instances[i].version,
                    display: this.formatVersion(this.selectedVersion, this.app.instances[i].revision)
                }
            }
        }
    },

    methods: {
        sortInstances: function (instances) {
            let temp = obj.copy(instances)
            
            return temp.sort(function(a,b) {
                return a.order - b.order;
            });
        },
        getCommits(version) {
            this.commits = [];
            this.loadingChanges = true

            let that = this
            fetch('/v1/apps/'+this.app.name+"/version/range/"+this.instance.version+"/to/"+version+"/commits")
            .then(function(response) {
                return response.json()
            })
            .then(function(data) {
                if (data.message) {
                    return Promise.reject(new Error(data.message))
                }

                return Promise.resolve(data);
            })
            .then(function(commits) {
                that.commits = commits;
            })
            .catch(function(e) {
                that.error = e.message;
            })
            .finally(function() {
                that.loadingChanges = false
            });
        },
        deploy: function (instance) {
            this.error = ""
            this.deploying = true
           
            let that = this

            let body = {
                "version": this.selectedVersion
            }

            if (this.override.env) {
                body.override = true
                body.env = env.parse(this.environment, {});
                body.secrets = env.parse(this.secrets, {});
            }

            let ver = this.versions.data[this.selectedVersion]
            if (ver.type === BUILD_TYPE_IMAGE) {
                body.imageTag = this.selectedVersion;
            }

            return fetch('/v1/apps/' + this.app.name + "/instances/" + instance + "/deploy/" + this.selectedSource + "/" + ver.revision, {
                method: "POST",
                body: JSON.stringify(body),
                headers: {
                    'Accept': 'application/json',
                    'Content-Type': 'application/json' 
                }
            })
            .then(function(response) {
                return response.json()
            })
            .then(function(data) { 
                if (data.message) {
                    that.error = data.message;
                    that.deploying = false
                } else {
                    that.$parent.modal.deploy.show = false
                }
            })
        },
        loadVersions: function (instance) {
            return fetch('/v1/apps/' + this.app.name + "/instances/" + instance + "/registry")
            .then(function(response) {
                return response.json()
            })
        },
        validateDeployCode: function (evt) {
            this.deployAuthorized = (this.instance.deployCode == evt.target.value)
        },
        overrideEnv: function(evt) {
            this.override.env = evt.target.checked;
        },
        formatVersion(version, revision, type) {
            return type === BUILD_TYPE_IMAGE
              ? version
              : revision === 0 
                ? version
                : version + " (" + revision + ")"
        },
        formatEnv(env) {
            let envFile = ""

            for (let key in env) {
                envFile = `${envFile}${key}=${env[key]}\n`
            }

            return envFile
        },
        willPropagate() {
            for(let i in this.app.instances) {
                if (this.app.instances[i].task.registry == this.instance.name && this.app.instances[i].autoPropagate) {
                    return true
                }
            }

            return false
        }
    }
})