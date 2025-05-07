pipeline 

{
	agent { label 'build' }
	parameters
	{
		string(name: 'BRANCH', defaultValue: 'master', description: 'The Name of the git branch from where you need to fetch code')
		string(name: 'BUILD_ID_OVERRIDE', defaultValue: '', description: 'This is appended to version string, eg. "123" resolves to version: 24.3.00_DROP-123 \n(defaults to current job id if left empty. Set to "none" to not add a build id to version)')
        booleanParam(name: 'GIT_CHECKOUT', defaultValue: 'true', description: 'Select to checkout yours code from git')
        booleanParam(name: 'CLEAN_OLD_IMAGES', defaultValue: 'false', description: 'Select to clean old docker images from build machine')
        booleanParam(name: 'BUILD', defaultValue: 'true', description: 'Building Go binaries')
        booleanParam(name: 'SET_BUILD_VERSION', defaultValue: 'true', description: 'set build version')
        booleanParam(name: 'CREATE_DOCKER_IMAGE', defaultValue: 'true', description: 'Create Docker Images')
        booleanParam(name: 'PUSH_DOCKER_IMAGE', defaultValue: 'true', description: 'push docker images to pune docker registry')
        booleanParam(name: 'PUSH_DOCKER_IMAGE_PSG', defaultValue: 'false', description: 'Push Docker images to PSG-Harbor-pun docker registry')
		booleanParam(name: 'PUSH_ANOMALY_IMAGE_EPD', defaultValue: 'false', description: 'Push Docker images to PSG-Harbor-pun docker registry')
		booleanParam(name: 'PUSH_CLASSIFICATION_IMAGE_EPD', defaultValue: 'false', description: 'Push Docker images to PSG-Harbor-pun docker registry')
        booleanParam(name: 'PACKAGING', defaultValue: 'false', description: 'Packag the deployment')
        booleanParam(name: 'UNIT_TEST', defaultValue: 'true', description: 'Unit Test Coverage')
	}
	tools
	{
		go "Go 1.24.2 Linux"
		nodejs "nodejs"
	}

	environment {
		GOCACHE="/root/.cache/go-build"
		GOMODCACHE="/root/go/pkg/mod"
		GOROOT="/usr/local/go/bin/go"
		PROJECT_NAME="Hedge"
		BUILD="${env.PROJECT_NAME}:${env.BUILD_NUMBER}"
		RELEASE_VERSION="1.0.00"
		IMG_TYPE="DROP"     //"DROP" for dev/nightly builds, "RC" for pre-prod, "CA" or "GA" for prod
		BUILD_ID="${params.BUILD_ID_OVERRIDE == '' ? env.BUILD_NUMBER : (params.BUILD_ID_OVERRIDE == 'none' ? '' : params.BUILD_ID_OVERRIDE)}"
		BUILD_SEP="${BUILD_ID == '' ? '' : '-'}"
		//VERSION_STRING="${RELEASE_VERSION}_${IMG_TYPE}${BUILD_SEP}${BUILD_ID}" //eg. 24.3.00_DROP-1 or 24.1.00_CA if build_id_override = none
		VERSION_STRING="${RELEASE_VERSION}_latest"
		BRANCH_NAME="master"
		EMAIL_NOTIFY="edgedev@bmc.com"
		SAMBA_SERVER="clm-aus-w71olm"
	}

	stages {
	 	stage('Start') {
			steps {
				echo 'Jenkins job started for the latest code check-in to GIT'
				emailext body: '''${SCRIPT, template="groovy-html.template"}''',
				mimeType: 'text/html',
				subject: "Job \'${JOB_NAME}\' - (${BUILD_NUMBER}) - (${VERSION_STRING}) - STARTED",
				to: "${EMAIL_NOTIFY}"
			}
		}

		stage('Git Checkout Branch') {
            when {
                expression {
                    return params.GIT_CHECKOUT
                }
            }
			steps {
                
                echo "Cloning open-edge GIT repo - Branch: ${params.BRANCH}"
                git credentialsId: 'gitcredentials',branch: "${params.BRANCH}", url: 'https://github.bmc.com/CTO-BIL/open-edge.git'
			}
		}

		stage('Cleanup old Docker Images'){
            when {
                expression {
                    def cause = currentBuild.getBuildCauses('hudson.model.Cause$UserIdCause')
                    echo "userName: ${cause.userName}"
                    if  ("${cause.userName}"=="[]"){
                        return true
                    }else{
                        return params.CLEAN_OLD_IMAGES
                    }
                }
            }
			steps {
                echo 'Cleaning up old docker images'
                sh"""
                    set +e
					make clean-docker
					rm -f /tmp/*.tar
					echo "removing service files so we re-build even if nothing changed"
					find . -maxdepth 1 -type f -size 0 -print -exec rm {} +
					df -h
					# Run full cleanup on Saturdays (day number 6 in cron notation)
					if [ "\$(date +%u)" -eq "6" ]; then
                        echo "Running full disk cleanup on a Saturday"
						# docker images -q | xargs docker rmi -f >/dev/null
						docker system prune -f >/dev/null
						echo "Cleanup done!"
						df -h
					else
						docker system prune -f >/dev/null
						# echo "Skipping disk cleanup.."
					fi
					set -e
				""" 
			}
		}

		stage('Build'){
            when {
                expression {
                    return params.BUILD
                }
            }
			steps {
				echo 'Building Go binaries'
				sh 'cd ${WORKSPACE}'
				sh 'make clean'
				sh 'make build'
			}
		}
		stage('Set build version') {
            when {
                expression {
                    return params.SET_BUILD_VERSION
                }
            }
			steps {
				sh"""
					echo "Set docker build version as \$VERSION_STRING"
					echo "\$VERSION_STRING" > ./VERSION
				"""
			}
		}
		stage('Create Docker Images') {
            when {
                expression {
                    return params.CREATE_DOCKER_IMAGE
                }
            }
			steps {
				echo 'Printing Docker version of the build VM'
				sh 'docker --version'
				echo 'Delete submodule lock file if it exists'
				sh 'rm -f ${WORKSPACE}/.git/modules/external/edgex-go-submodule/edgex-go/index.lock'
				sleep 1
				echo 'Creating docker images for BMC HEDGE & EDGEX Microservices'
				sh 'make -j 2 docker'
			}
		}
		stage('Push Docker Pun-Harbor') {
            when {
                expression {
                    return params.PUSH_DOCKER_IMAGE
                }
            }
			steps {
				echo 'Pushing Hedge Stack Images to Harbor pun-harbor-reg1.bmc.com'
				withCredentials([usernamePassword(credentialsId: 'Harbor', passwordVariable: 'HBR_PWD', usernameVariable: 'HBR_USER')])
				{
					// Docker login
					sh 'echo ${HBR_PWD} | docker login pun-harbor-reg1.bmc.com -u ${HBR_USER} --password-stdin'
				}
				sh 'echo "Build ID is ${BUILD_ID}" '
				// Push images to R&D Harbor with tags:
				//		1. tag from VERSION file = <RELEASE VER>_DROP/RC-<BUILD ID>. Eg. 24.1.00_DROP-123 for dev. 24.1.00_RC-123 for prod),
				//		2. <RELEASE VER>_latest. Eg. 24.1.00_latest,
				//		3. latest
				sh 'make -j 2 push -e REGISTRY=pun-harbor-reg1.bmc.com/iot/'
				//sh 'make -j 2 push -e REGISTRY=pun-harbor-reg1.bmc.com/iot/ -e NEW_DOCKER_TAG=${RELEASE_VERSION}_latest'
				//sh 'make -j 2 push -e REGISTRY=pun-harbor-reg1.bmc.com/iot/ -e NEW_DOCKER_TAG=latest'
				sh 'make -j 2 push-edgex-images -e REGISTRY=pun-harbor-reg1.bmc.com/iot/'
				sh 'make -j 2 push-edgex-images -e REGISTRY=pun-harbor-reg1.bmc.com/iot/ -e NEW_DOCKER_TAG=${RELEASE_VERSION}_latest'
				sh 'make -j 2 push-edgex-images -e REGISTRY=pun-harbor-reg1.bmc.com/iot/ -e NEW_DOCKER_TAG=latest'
				sh 'make -j 2 push-opt-images -e REGISTRY=pun-harbor-reg1.bmc.com/iot/'
				sh 'make -j 2 push-opt-images -e REGISTRY=pun-harbor-reg1.bmc.com/iot/ -e NEW_DOCKER_TAG=${RELEASE_VERSION}_latest'
				sh 'make -j 2 push-opt-images -e REGISTRY=pun-harbor-reg1.bmc.com/iot/ -e NEW_DOCKER_TAG=latest'
            }
		}
		stage('Push Docker PSG-Harbor-pun and Nexus') {
            when {
                expression {
                    def cause = currentBuild.getBuildCauses('hudson.model.Cause$UserIdCause')
                    if  ("${cause.userName}"=="[]"){
                        return true
                    }else{
                        return params.PUSH_DOCKER_IMAGE_PSG
                    }
                }
            }
			steps {
				echo 'Pushing Hedge Stack Images to Harbor psg-hrbr-pun.bmc.com and Nexus'
				withCredentials([usernamePassword(credentialsId: 'Harbor-PSG', passwordVariable: 'HBRPSG_PWD', usernameVariable: 'HBRPSG_USER')])
				{
					// Docker login
					sh 'echo ${HBRPSG_PWD} | docker login psg-hrbr-pun.bmc.com -u ${HBRPSG_USER} --password-stdin'
				}
				withCredentials([usernamePassword(credentialsId: 'NEXUS-PSG', passwordVariable: 'PSG_NEXUS_PWD', usernameVariable: 'PSG_NEXUS_USER')])
				{
					// Push (1) "latest" tagged images to PSG harbor registry and (2) image tar's to Nexus share
					sh 'make -j 5 push -e REGISTRY=psg-hrbr-pun.bmc.com/iot/ -e NEW_DOCKER_TAG=latest -e nexus_repo_pass=${PSG_NEXUS_PWD} -e nexus_repo_user=${PSG_NEXUS_USER}'
					sh 'make -j 5 push-edgex-images -e REGISTRY=psg-hrbr-pun.bmc.com/iot/ -e NEW_DOCKER_TAG=latest'
				}
			}
		}
		stage('Push Anomaly Training Image to EPD & Pun-Harbor') {
            when {
                expression {
					 def cause = currentBuild.getBuildCauses('hudson.model.Cause$UserIdCause')
                    if  ("${cause.userName}"=="[]"){
                        return true
                    }else{
                        return params.PUSH_ANOMALY_IMAGE_EPD
                    }
                }
            }        
			steps {
				script{
                        // Read the contents of the VERSION file and store it in a variable
                        def IMAGE_VERSION = readFile('VERSION').trim()
                        // Print the version to verify
                        echo "Version: ${IMAGE_VERSION}"
                        withCredentials([usernamePassword(credentialsId: 'Harbor', passwordVariable: 'HBR_PWD', usernameVariable: 'HBR_USER')])
                        {
                            // Docker login Pun-Harbor
                            sh 'echo ${HBR_PWD} | docker login pun-harbor-reg1.bmc.com -u ${HBR_USER} --password-stdin'
                        }
                        withCredentials([usernamePassword(credentialsId: 'Harbor-EPD', passwordVariable: 'HBREPD_PWD', usernameVariable: 'HBREPD_USER')])
                        {
                            // Docker login EPD
                            sh 'echo ${HBREPD_PWD} | docker login phx-epddtr-prd.bmc.com -u ${HBREPD_USER} --password-stdin'
                        }
                        sh"""
                            docker tag hedge-ml-trg-anomaly-autoencoder:${IMAGE_VERSION} pun-harbor-reg1.bmc.com/iot/lpade:hedge-ml-trg-anomaly-autoencoder
                            docker push pun-harbor-reg1.bmc.com/iot/lpade:hedge-ml-trg-anomaly-autoencoder
                            docker tag hedge-ml-trg-anomaly-autoencoder:${IMAGE_VERSION} phx-epddtr-prd.bmc.com/bmc/lpade:hedge-ml-trg-anomaly-autoencoder
                            docker push phx-epddtr-prd.bmc.com/bmc/lpade:hedge-ml-trg-anomaly-autoencoder
                        """
                }
			}
		}
		stage('Push Classification Training Image to EPD & Pun-Harbor') {
            when {
                expression {
					 def cause = currentBuild.getBuildCauses('hudson.model.Cause$UserIdCause')
                    if  ("${cause.userName}"=="[]"){
                        return true
                    }else{
                        return params.PUSH_CLASSIFICATION_IMAGE_EPD
                    }
                }
            }
			steps {
				script{
                        // Read the contents of the VERSION file and store it in a variable
                        def IMAGE_VERSION = readFile('VERSION').trim()
                        // Print the version to verify
                        echo "Version: ${IMAGE_VERSION}"
                        withCredentials([usernamePassword(credentialsId: 'Harbor', passwordVariable: 'HBR_PWD', usernameVariable: 'HBR_USER')])
                        {
                            // Docker login Pun-Harbor
                            sh 'echo ${HBR_PWD} | docker login pun-harbor-reg1.bmc.com -u ${HBR_USER} --password-stdin'
                        }
                        withCredentials([usernamePassword(credentialsId: 'Harbor-EPD', passwordVariable: 'HBREPD_PWD', usernameVariable: 'HBREPD_USER')])
                        {
                            // Docker login EPD
                            sh 'echo ${HBREPD_PWD} | docker login phx-epddtr-prd.bmc.com -u ${HBREPD_USER} --password-stdin'
                        }
                        sh"""
                            docker tag hedge-ml-trg-classification-randomforest:${IMAGE_VERSION} pun-harbor-reg1.bmc.com/iot/lpade:hedge-ml-trg-classification-randomforest
                            docker push pun-harbor-reg1.bmc.com/iot/lpade:hedge-ml-trg-classification-randomforest
                            docker tag hedge-ml-trg-classification-randomforest:${IMAGE_VERSION} phx-epddtr-prd.bmc.com/bmc/lpade:hedge-ml-trg-classification-randomforest
                            docker push phx-epddtr-prd.bmc.com/bmc/lpade:hedge-ml-trg-classification-randomforest
                        """
                }
			}
		}
		stage('Push Timeseries Training Image to EPD & Pun-Harbor') {
            when {
                expression {
					 def cause = currentBuild.getBuildCauses('hudson.model.Cause$UserIdCause')
                    if  ("${cause.userName}"=="[]"){
                        return true
                    }else{
                        return params.PUSH_TIMESERIES_IMAGE_EPD
                    }
                }
            }
			steps {
				script{
                        // Read the contents of the VERSION file and store it in a variable
                        def IMAGE_VERSION = readFile('VERSION').trim()
                        // Print the version to verify
                        echo "Version: ${IMAGE_VERSION}"
                        withCredentials([usernamePassword(credentialsId: 'Harbor', passwordVariable: 'HBR_PWD', usernameVariable: 'HBR_USER')])
                        {
                            // Docker login Pun-Harbor
                            sh 'echo ${HBR_PWD} | docker login pun-harbor-reg1.bmc.com -u ${HBR_USER} --password-stdin'
                        }
                        withCredentials([usernamePassword(credentialsId: 'Harbor-EPD', passwordVariable: 'HBREPD_PWD', usernameVariable: 'HBREPD_USER')])
                        {
                            // Docker login EPD
                            sh 'echo ${HBREPD_PWD} | docker login phx-epddtr-prd.bmc.com -u ${HBREPD_USER} --password-stdin'
                        }
                        sh"""
                            docker tag hedge-ml-trg-timeseries-multivariate-deepvar:${IMAGE_VERSION} pun-harbor-reg1.bmc.com/iot/lpade:hedge-ml-trg-timeseries-multivariate-deepvar
                            docker push pun-harbor-reg1.bmc.com/iot/lpade:hedge-ml-trg-timeseries-multivariate-deepvar
                            docker tag hedge-ml-trg-timeseries-multivariate-deepvar:${IMAGE_VERSION} phx-epddtr-prd.bmc.com/bmc/lpade:hedge-ml-trg-timeseries-multivariate-deepvar
                            docker push phx-epddtr-prd.bmc.com/bmc/lpade:hedge-ml-trg-timeseries-multivariate-deepvar
                        """
                }
			}
		}
		stage('Push Regression Training Image to EPD & Pun-Harbor') {
            when {
                expression {
					 def cause = currentBuild.getBuildCauses('hudson.model.Cause$UserIdCause')
                    if  ("${cause.userName}"=="[]"){
                        return true
                    }else{
                        return params.PUSH_REGRESSION_IMAGE_EPD
                    }
                }
            }
			steps {
				script{
                        // Read the contents of the VERSION file and store it in a variable
                        def IMAGE_VERSION = readFile('VERSION').trim()
                        // Print the version to verify
                        echo "Version: ${IMAGE_VERSION}"
                        withCredentials([usernamePassword(credentialsId: 'Harbor', passwordVariable: 'HBR_PWD', usernameVariable: 'HBR_USER')])
                        {
                            // Docker login Pun-Harbor
                            sh 'echo ${HBR_PWD} | docker login pun-harbor-reg1.bmc.com -u ${HBR_USER} --password-stdin'
                        }
                        withCredentials([usernamePassword(credentialsId: 'Harbor-EPD', passwordVariable: 'HBREPD_PWD', usernameVariable: 'HBREPD_USER')])
                        {
                            // Docker login EPD
                            sh 'echo ${HBREPD_PWD} | docker login phx-epddtr-prd.bmc.com -u ${HBREPD_USER} --password-stdin'
                        }
                        sh"""
                            docker tag hedge-ml-trg-regression-lightgbm:${IMAGE_VERSION} pun-harbor-reg1.bmc.com/iot/lpade:hedge-ml-trg-regression-lightgbm
                            docker push pun-harbor-reg1.bmc.com/iot/lpade:hedge-ml-trg-regression-lightgbm
                            docker tag hedge-ml-trg-regression-lightgbm:${IMAGE_VERSION} phx-epddtr-prd.bmc.com/bmc/lpade:hedge-ml-trg-regression-lightgbm
                            docker push phx-epddtr-prd.bmc.com/bmc/lpade:hedge-ml-trg-regression-lightgbm
                        """
                }
			}
		}
		stage('Packaging') {
            when {
                expression {
                    def cause = currentBuild.getBuildCauses('hudson.model.Cause$UserIdCause')
                    if  ("${cause.userName}"=="[]"){
                        return true
                    }else{
                        return params.PACKAGING
                    }
                }
            }
			steps('Create package') {
				withCredentials([usernamePassword(credentialsId: 'bmcadmin', passwordVariable: 'BMCPWD', usernameVariable: 'BMCADMIN')])
				{
					sh"""
						set +e
						// Create package files
						make package

						// Push the installers to Samba share
						basedir=/home/hedge/hedge_installer

						pkgdir=\$(date +"%Y%m%d")_\${VERSION_STRING}
						sshpass -p "\${BMCPWD}" rsync -v -e "ssh -o StrictHostKeychecking=no" ./package/* root@\${SAMBA_SERVER}:\${basedir}/\${pkgdir}/ && echo -e "Successfully copied package files to \${SAMBA_SERVER}:\${basedir}/\${pkgdir}\n";
						sshpass -p "\${BMCPWD}" rsync -v -e "ssh -o StrictHostKeychecking=no" ./package/* root@\${SAMBA_SERVER}:\${basedir}/latest/ && echo -e "Successfully copied package files to \${SAMBA_SERVER}:\${basedir}/latest\n";
						set -e
					"""
				}
			}
		}
		stage('Test') {
             when {
                expression {
                    return params.UNIT_TEST
                }
            }
			steps('Unit Test Coverage') {
				echo 'Testing Phase'

				sh 'echo "Build docker image > Run Unit Tests > generate code coverage reports"'
				sh 'make codecoverage -e DOCKER_TAG=latest'

				sh 'echo "Runs sonar-scanner cli for code scanning and reporting based on above coverage reports - for BE, UI and ALL"'
				withCredentials([
					string(credentialsId: 'sonar_token_go', variable: 'sonar_token_go'),
					string(credentialsId: 'sonar_token_ui', variable: 'sonar_token_ui'),
					string(credentialsId: 'sonar_token_py', variable: 'sonar_token_py'),
					string(credentialsId: 'sonar_token_all', variable: 'sonar_token_all')])
				{
					sh 'docker run --network=host -e sonar_token_go=${sonar_token_go} -e sonar_token_ui=${sonar_token_ui} -e sonar_token_py=${sonar_token_py} -e sonar_token_all=${sonar_token_all} hedge-test-coverage'
				}
		    }
        }
    }
	post {
		always {
			echo 'clean up go binaries'
			sh 'make clean'
		}

		success {
			echo 'Jenkins job executed successfully'
			emailext body: '''${SCRIPT, template="groovy-html.template"}''',
			mimeType: 'text/html',
			subject: "Job \'${JOB_NAME}\' - (${BUILD_NUMBER}) - (${VERSION_STRING}) - SUCCESS",
			to: "${EMAIL_NOTIFY}"
		}

		failure {
			echo 'Jenkins job execution failed'
			emailext body: '''${SCRIPT, template="groovy-html.template"}''',
			mimeType: 'text/html',
			attachLog: true,
			subject: "Job \'${JOB_NAME}\' - (${BUILD_NUMBER}) - (${VERSION_STRING}) - FAILED",
			to: "${EMAIL_NOTIFY}"
		}
	}
}
