package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"math"
	"net"
	"os"
	"strings"
)

var connection net.Conn = nil

type auth1 struct {
	Auth1   string   `xml:"auth1"`
	XMLName xml.Name `xml:"boinc_gui_rpc_request"`
}

type auth2 struct {
	XMLName   xml.Name `xml:"boinc_gui_rpc_request"`
	NonceHash string   `xml:"auth2>nonce_hash"`
}

type Project struct {
	XMLName         xml.Name `xml:"project"`
	ProjectName     string   `xml:"project_name"`
	UserTotalCredit float64  `xml:"user_total_credit"`
	UserAvgCredit   float64  `xml:"user_expavg_credit"`
	HostTotalCredit float64  `xml:"host_total_credit"`
	HostAvgCredit   float64  `xml:"host_expavg_credit"`
	NJobsSuccess    int      `xml:"njobs_success"`
	NJobsError      int      `xml:"njobs_error"`
	ElapsedTime     float64  `xml:"elapsed_time"`
	MasterUrl       string   `xml:"master_url"`
}

type Result struct {
	XMLName                xml.Name   `xml:"result"`
	EstimatedTimeRemaining float64    `xml:"estimated_cpu_time_remaining"`
	FinalElapsedTime       float64    `xml:"final_elapsed_time"`
	FinalCPUTime           float64    `xml:"final_cpu_time"`
	Platform               string     `xml:"platform"`
	Name                   string     `xml:"name"`
	WUName                 string     `xml:"wu_name"`
	State                  int        `xml:"state"`
	Activetask             ActiveTask `xml:"active_task"`
	ProjectUrl             string     `xml:"project_url"`
	ReadyToReport          *struct{}  `xml:"ready_to_report"` // is nil when not ready, not nil when ready
}

type ActiveTask struct {
	XMLName           xml.Name `xml:"active_task"`
	State             int      `xml:"active_task_state"`
	CheckpointCPUTime float64  `xml:"checkpoint_cpu_time"`
	ElapsedTime       float64  `xml:"elapsed_time"`
	WorkingSetSize    float64  `xml:"working_set_size"`
	ProgressRate      float64  `xml:"progress_rate"`
}

type App struct {
	XMLName          xml.Name `xml:"app"`
	UserFriendlyName string   `xml:"user_friendly_name"`
	NonCpuIntensive  int      `xml:"non_cpu_intensive"`
}

type AppVersion struct {
	XMLName    xml.Name `xml:"app_version"`
	AppName    string   `xml:"app_name"`
	VersionNum int      `xml:"version_num"`
	Platform   string   `xml:"platform"`
	AvgNcpus   int      `xml:"avg_ncpus"`
	Flops      float64  `xml:"flops"`
}

type WorkUnit struct {
	XMLName        xml.Name `xml:"workunit"`
	Name           string   `xml:"name"`
	AppName        string   `xml:"app_name"`
	RscFpopsEst    float64  `xml:"rsc_fpops_est"`
	RscFpopsBound  float64  `xml:"rsc_fpops_bound"`
	RscMemoryBound float64  `xml:"rsc_memory_bound"`
	RscDiskBound   float64  `xml:"rsc_disk_bound"`
}

type simpleGuiInfo struct {
	XMLName xml.Name `xml:"get_simple_gui_info"`
}

type GetState struct {
	XMLName xml.Name `xml:"get_state"`
}

type simpleGuiInfoReply struct {
	XMLName       xml.Name `xml:"boinc_gui_rpc_reply"`
	SimpleGuiInfo struct {
		Projects []Project `xml:"project"`
		Results  []Result  `xml:"result"`
	} `xml:"simple_gui_info"`
}

type ClientStateReply struct {
	XMLName     xml.Name `xml:"boinc_gui_rpc_reply"`
	ClientState struct {
		// HostInfo
		// Coprocs
		// NetState
		// TimeStats
		Projects []Project `xml:"project"`
		Results  []Result  `xml:"result"`
		Apps     []App     `xml:"app"`
		//AppVersions []AppVersion `xml:"app_version"`
		WorkUnits []WorkUnit `xml:"workunit"`
	} `xml:"client_state"`
}

type nonce struct {
	XMLName xml.Name `xml:"boinc_gui_rpc_reply"`
	Nonce   string   `xml:"nonce"`
}

func findProjectByUrl(result *Result, projects []Project) *Project {
	for _, project := range projects {
		if project.MasterUrl == result.ProjectUrl {
			return &project
		}
	}

	return nil
}

func findWUbyName(unitname string, units []WorkUnit) *WorkUnit {
	unitname = string(unitname[0:strings.LastIndex(unitname, "_")])
	for _, wu := range units {
		//fmt.Printf("Comparing %s == %s\n", unitname, wu.Name)
		if wu.Name == unitname {
			return &wu
		}
	}

	return nil
}

func countTasksOfProject(project *Project, results []Result) int {
	count := 0
	for _, result := range results {
		if result.ProjectUrl == project.MasterUrl && result.Activetask.State == 1 {
			count++
		}
	}

	return count
}

func send(object interface{}) error {
	enc, err := xml.MarshalIndent(object, "> ", "  ")
	if err != nil {
		fmt.Errorf("Error marshaling: %v\n", err)
		return err
	}
	fmt.Printf("Sending: \n%s\n", enc)
	enc2 := append(enc, 0x03)
	fmt.Fprintf(connection, "%s", enc2)
	return nil
}

func recv(objectOut interface{}) {
	fmt.Printf("Waiting for recv")
	message, _ := bufio.NewReader(connection).ReadString(0x03)
	fmt.Printf("Received msg: \n%s\n", message)
	if objectOut != nil {
		err := xml.Unmarshal([]byte(message), objectOut)
		if err != nil {
			fmt.Errorf("Error unmarshaling: %v\n", err)
		}
	}
}

func main() {
        if len(os.Args) == 1 {
                fmt.Printf("Not enough arguments! Use %s <passkey>\n", os.Args[0])
                return
        }
        passkey := os.Args[1]
	connection, _ = net.Dial("tcp", "127.0.0.1:31416")
	authMsg := &auth1{}
	send(authMsg)
	nonceMsg := &nonce{}
	recv(nonceMsg)
	fmt.Printf("Nonce: %s", nonceMsg.Nonce)
	password := nonceMsg.Nonce + passkey
	calculated := md5.Sum([]byte(password))
	var calculated2 []byte = calculated[:]
	send(&auth2{NonceHash: hex.EncodeToString(calculated2)})
	recv(nil)

	foo := GetState{}
	clientStateReply := ClientStateReply{}
	send(&foo)
	recv(&clientStateReply)

	fmt.Printf("%+v\n", clientStateReply)
	f, err := os.Create("/var/lib/prometheus/node-exporter/boinc.prom.tmp")
	if err != nil {
		fmt.Errorf("Error opening a file: %v", err)
	}

	fmt.Fprintf(f, "# TYPE boinc_client_user_total_credit counter\n")
	for _, project := range clientStateReply.ClientState.Projects {
		fmt.Fprintf(f, "boinc_client_user_total_credit{project=\"%s\"} %f\n", project.ProjectName, math.Round(project.UserTotalCredit))
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "# TYPE boinc_client_host_total_credit counter\n")
	for _, project := range clientStateReply.ClientState.Projects {
		fmt.Fprintf(f, "boinc_client_host_total_credit{project=\"%s\"} %f\n", project.ProjectName, math.Round(project.HostTotalCredit))
	}

	fmt.Fprintf(f, "\n")
	fmt.Fprint(f, "# TYPE boinc_client_jobs_success counter\n")
	for _, project := range clientStateReply.ClientState.Projects {
		fmt.Fprintf(f, "boinc_client_jobs_success{project=\"%s\"} %d\n", project.ProjectName, project.NJobsSuccess)
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprint(f, "# TYPE boinc_client_jobs_error counter\n")
	for _, project := range clientStateReply.ClientState.Projects {
		fmt.Fprintf(f, "boinc_client_jobs_error{project=\"%s\"} %d\n", project.ProjectName, project.NJobsError)
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprint(f, "# TYPE boinc_client_host_avg_credit gauge\n")
	for _, project := range clientStateReply.ClientState.Projects {
		fmt.Fprintf(f, "boinc_client_host_avg_credit{project=\"%s\"} %f\n", project.ProjectName, math.Round(project.HostAvgCredit))
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprint(f, "# TYPE boinc_client_user_avg_credit gauge\n")
	for _, project := range clientStateReply.ClientState.Projects {
		fmt.Fprintf(f, "boinc_client_user_avg_credit{project=\"%s\"} %f\n", project.ProjectName, math.Round(project.UserAvgCredit))
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprint(f, "# TYPE boinc_client_project_elapsed_time counter\n")
	for _, project := range clientStateReply.ClientState.Projects {
		fmt.Fprintf(f, "boinc_client_project_elapsed_time{project=\"%s\"} %f\n", project.ProjectName, math.Round(project.ElapsedTime))
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprint(f, "# TYPE boinc_client_task_time_remaining gauge\n")
	for _, result := range clientStateReply.ClientState.Results {
		fmt.Fprintf(f, "boinc_client_task_time_remaining{state=\"%d\",wuname=\"%s\"} %f\n", result.Activetask.State, result.WUName, math.Round(result.EstimatedTimeRemaining))
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprint(f, "# TYPE boinc_client_task_final_cpu_time gauge\n")
	for _, result := range clientStateReply.ClientState.Results {
		readyToReport := "no"
		if result.ReadyToReport != nil {
			readyToReport = "yes"
		}
		fmt.Fprintf(f, "boinc_client_task_final_cpu_time{wuname=\"%s\",ready_to_upload=\"%s\"} %f\n", result.WUName, readyToReport, math.Round(result.FinalCPUTime))
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprint(f, "# TYPE boinc_client_task_working_set_size gauge\n")
	for _, result := range clientStateReply.ClientState.Results {
		fmt.Fprintf(f, "boinc_client_task_working_set_size{wuname=\"%s\",state=\"%d\"} %f\n", result.WUName, result.Activetask.State, math.Round(result.Activetask.WorkingSetSize))
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "# TYPE boinc_client_project_active_jobs gauge\n")
	for _, project := range clientStateReply.ClientState.Projects {
		fmt.Fprintf(f, "boinc_client_project_active_jobs{project=\"%s\"} %d\n", project.ProjectName, countTasksOfProject(&project, clientStateReply.ClientState.Results))
	}

	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "# TYPE boinc_client_task_rsc_memory_bound gauge\n")
	for _, task := range clientStateReply.ClientState.Results {
		wu := findWUbyName(task.Name, clientStateReply.ClientState.WorkUnits)
		if wu == nil {
			fmt.Printf("No such WU task: %s\n", task.Name)
			continue
		}
		fmt.Fprintf(f, "boinc_client_task_rsc_memory_bound{wuname=\"%s\",state=\"%d\"} %f\n", task.Name, task.Activetask.State, wu.RscMemoryBound)
	}

	f.Sync()
	f.Close()
	if err := os.Rename("/var/lib/prometheus/node-exporter/boinc.prom.tmp", "/var/lib/prometheus/node-exporter/boinc.prom"); err != nil {
		fmt.Errorf("Error renaming prom file: %v", err)
	}
}
