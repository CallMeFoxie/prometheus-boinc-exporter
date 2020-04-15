package main
import (
        "net"
        "fmt"
        "bufio"
        "encoding/xml"
        "encoding/hex"
        "crypto/md5"
	"math"
	"os"
)

var connection net.Conn = nil

type auth1 struct {
        Auth1 string `xml:"auth1"`
        XMLName xml.Name `xml:"boinc_gui_rpc_request"`
}

type auth2 struct {
        XMLName xml.Name `xml:"boinc_gui_rpc_request"`
        NonceHash string `xml:"auth2>nonce_hash"`
}

type Project struct {
	XMLName xml.Name `xml:"project"`
	ProjectName string `xml:"project_name"`
	UserTotalCredit float64 `xml:"user_total_credit"`
	UserAvgCredit float64 `xml:"user_expavg_credit"`
	HostTotalCredit float64 `xml:"host_total_credit"`
	HostAvgCredit float64 `xml:"host_expavg_credit"`
	NJobsSuccess int `xml:"njobs_success"`
	NJobsError int `xml:"njobs_error"`
	ElapsedTime float64 `xml:"elapsed_time"`
	MasterUrl string `xml:"master_url"`
}

type Result struct {
	XMLName xml.Name `xml:"result"`
	EstimatedTimeRemaining float64 `xml:"estimated_cpu_time_remaining"`
	FinalElapsedTime float64 `xml:"final_elapsed_time"`
	FinalCPUTime float64 `xml:"final_cpu_time"`
	Platform string `xml:"platform"`
	Name string `xml:"name"`
	WUName string `xml:"wu_name"`
	State int `xml:"state"`
	Activetask ActiveTask `xml:"active_task"`
	ProjectUrl string `xml:"project_url"`
}

type ActiveTask struct {
	XMLName xml.Name `xml:"active_task"`
	State int `xml:"active_task_state"`
	CheckpointCPUTime float64 `xml:"checkpoint_cpu_time"`
	ElapsedTime float64 `xml:"elapsed_time"`
	WorkingSetSize float64 `xml:"working_set_size"`
	ProgressRate float64 `xml:"progress_rate"`
}

type simpleGuiInfo struct {
        XMLName xml.Name `xml:"get_simple_gui_info"`
}

type simpleGuiInfoReply struct {
	XMLName xml.Name `xml:"boinc_gui_rpc_reply"`
	SimpleGuiInfo struct {
		Projects []Project `xml:"project"`
		Results []Result `xml:"result"`
	} `xml:"simple_gui_info"`
}

type nonce struct {
        XMLName xml.Name `xml:"boinc_gui_rpc_reply"`
        Nonce string `xml:"nonce"`
}

func findProjectByUrl(result *Result, projects []Project) *Project {
	for _, project := range projects {
		if project.MasterUrl == result.ProjectUrl {
			return &project
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
        connection, _ = net.Dial("tcp", "127.0.0.1:31416")
        authMsg := &auth1{}
        send(authMsg)
        nonceMsg := &nonce{}
        recv(nonceMsg)
        fmt.Printf("Nonce: %s", nonceMsg.Nonce)
        password := nonceMsg.Nonce + "somepassword"
        calculated := md5.Sum([]byte(password))
        var calculated2 []byte = calculated[:]
        send(&auth2{NonceHash: hex.EncodeToString(calculated2)})
        recv(nil)
	info := &simpleGuiInfoReply{}
        send(&simpleGuiInfo{})
        recv(info)
	fmt.Printf("%+v\n", info)
	f, err := os.Create("/var/lib/prometheus/node-exporter/boinc.prom.tmp")
	if err != nil {
		fmt.Errorf("Error opening a file: %v", err)
	}

	fmt.Fprintf(f, "# TYPE boinc_client_user_total_credit counter\n")
	for _, project := range info.SimpleGuiInfo.Projects {
		fmt.Fprintf(f, "boinc_client_user_total_credit{project=\"%s\"} %f\n", project.ProjectName, math.Round(project.UserTotalCredit))
	}
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "# TYPE boinc_client_host_total_credit counter\n")
	for _, project := range info.SimpleGuiInfo.Projects {
		fmt.Fprintf(f, "boinc_client_host_total_credit{project=\"%s\"} %f\n", project.ProjectName, math.Round(project.HostTotalCredit))
	}

	fmt.Fprintf(f, "\n")
	fmt.Fprint(f, "# TYPE boinc_client_jobs_success counter\n")
	for _, project := range info.SimpleGuiInfo.Projects {
		fmt.Fprintf(f, "boinc_client_jobs_success{project=\"%s\"} %d\n", project.ProjectName, project.NJobsSuccess)
	}
        fmt.Fprintf(f, "\n")
        fmt.Fprint(f, "# TYPE boinc_client_jobs_error counter\n")
        for _, project := range info.SimpleGuiInfo.Projects {
                fmt.Fprintf(f, "boinc_client_jobs_error{project=\"%s\"} %d\n", project.ProjectName, project.NJobsError)
        }
        fmt.Fprintf(f, "\n")
        fmt.Fprint(f, "# TYPE boinc_client_host_avg_credit gauge\n")
        for _, project := range info.SimpleGuiInfo.Projects {
                fmt.Fprintf(f, "boinc_client_host_avg_credit{project=\"%s\"} %f\n", project.ProjectName, math.Round(project.HostAvgCredit))
        }
        fmt.Fprintf(f, "\n")
        fmt.Fprint(f, "# TYPE boinc_client_user_avg_credit gauge\n")
        for _, project := range info.SimpleGuiInfo.Projects {
                fmt.Fprintf(f, "boinc_client_user_avg_credit{project=\"%s\"} %f\n", project.ProjectName, math.Round(project.UserAvgCredit))
        }
        fmt.Fprintf(f, "\n")
        fmt.Fprint(f, "# TYPE boinc_client_project_elapsed_time counter\n")
        for _, project := range info.SimpleGuiInfo.Projects {
                fmt.Fprintf(f, "boinc_client_project_elapsed_time{project=\"%s\"} %f\n", project.ProjectName, math.Round(project.ElapsedTime))
        }
        fmt.Fprintf(f, "\n")
        fmt.Fprint(f, "# TYPE boinc_client_task_time_remaining gauge\n")
        for _, result := range info.SimpleGuiInfo.Results {
                fmt.Fprintf(f, "boinc_client_task_time_remaining{state=\"%d\",wuname=\"%s\"} %f\n", result.Activetask.State, result.WUName, math.Round(result.EstimatedTimeRemaining))
        }
        fmt.Fprintf(f, "\n")
        fmt.Fprint(f, "# TYPE boinc_client_task_final_cpu_time gauge\n")
        for _, result := range info.SimpleGuiInfo.Results {
                fmt.Fprintf(f, "boinc_client_task_final_cpu_time{wuname=\"%s\"} %f\n", result.WUName, math.Round(result.FinalCPUTime))
        }
        fmt.Fprintf(f, "\n")
        fmt.Fprint(f, "# TYPE boinc_client_task_working_set_size gauge\n")
        for _, result := range info.SimpleGuiInfo.Results {
                fmt.Fprintf(f, "boinc_client_task_working_set_size{wuname=\"%s\",state=\"%d\"} %f\n", result.WUName, result.Activetask.State, math.Round(result.Activetask.WorkingSetSize))
        }
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "# TYPE boinc_client_project_active_jobs gauge\n")
	for _, project := range info.SimpleGuiInfo.Projects {
		fmt.Fprintf(f, "boinc_client_project_active_jobs{project=\"%s\"} %d\n", project.ProjectName, countTasksOfProject(&project, info.SimpleGuiInfo.Results))
	}

	f.Sync()
	f.Close()
	if err := os.Rename("/var/lib/prometheus/node-exporter/boinc.prom.tmp", "/var/lib/prometheus/node-exporter/boinc.prom"); err != nil {
		fmt.Errorf("Error renaming prom file: %v", err)
	}
}

