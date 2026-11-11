-- +goose Up
-- Supplied beta listings, imported once. Availability is maintained manually.
INSERT INTO app.job_companies(name,category) VALUES
('Google','faang'),
('Amazon','faang'),
('Microsoft','big_tech'),
('Canva','big_tech'),
('Atlassian','big_tech'),
('TikTok','big_tech'),
('Vivcourt','quant'),
('Citadel Securities','quant'),
('Jump Trading','quant'),
('HRT','quant'),
('Jane Street','quant'),
('IMC','quant'),
('Optiver','quant'),
('Susquehanna','quant'),
('The Trade Desk','big_tech'),
('Bank of America','other'),
('MongoDB','big_tech'),
('Airwallex','big_tech'),
('Dolby','other')
ON CONFLICT (lower(trim(name))) DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineering Intern, Summer 2026','intern','','https://www.google.com/about/careers/applications/jobs/results/116505799294886598-software-engineering-intern-summer-2026','https://www.google.com/about/careers/applications/jobs/results/116505799294886598-software-engineering-intern-summer-2026',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Google')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'2026 Software Dev Engineer Intern (Location : Sydney)','intern','','https://www.amazon.jobs/en/jobs/3204846/2026-software-dev-engineer-intern-location-sydney','https://www.amazon.jobs/en/jobs/3204846/2026-software-dev-engineer-intern-location-sydney',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Amazon')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineer','grad','','https://apply.careers.microsoft.com/careers/job/1970393556749073','https://apply.careers.microsoft.com/careers/job/1970393556749073',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Microsoft')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineer Intern Frontend/Backend 2026/27','intern','','https://www.lifeatcanva.com/en/jobs/6000000000970210/software-engineer-intern-frontendbackend-202627/','https://www.lifeatcanva.com/en/jobs/6000000000970210/software-engineer-intern-frontendbackend-202627/',ARRAY['Hybrid'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Canva')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Security Engineer Intern, 2026 Summer Australia','intern','Australian Citizenship or PR','https://www.atlassian.com/company/careers/details/23862','https://www.atlassian.com/company/careers/details/23862',ARRAY['Sydney / Remote'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Atlassian')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineer Intern, 2026 Summer Australia','intern','Australian Citizenship or PR','https://www.atlassian.com/company/careers/details/23861','https://www.atlassian.com/company/careers/details/23861',ARRAY['Sydney / Remote'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Atlassian')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Backend Engineer Intern (TikTok LIVE) - 2027 Start (BS/MS)','intern','','https://lifeattiktok.com/search/7605436552185399605','https://lifeattiktok.com/search/7605436552185399605',ARRAY['Sydney'],'TikTok has many other roles, check out here https://lifeattiktok.com/search?job_category_id_list=&location_code_list=CT_244&recruitment_id_list=202%2C301&subject_id_list=&tag_id_list=&keyword=&limit=12&offset=0' FROM app.job_companies WHERE lower(trim(name))=lower('TikTok')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Backend Software Engineer Graduate (Trust and Safety Engineering) - 2027 Start (BS/MS)','grad','','https://lifeattiktok.com/search/7605166042878068997','https://lifeattiktok.com/search/7605166042878068997',ARRAY['Sydney'],'TikTok has many other roles, check out here https://lifeattiktok.com/search?job_category_id_list=&location_code_list=CT_244&recruitment_id_list=201&subject_id_list=&tag_id_list=&keyword=&limit=12&offset=0' FROM app.job_companies WHERE lower(trim(name))=lower('TikTok')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'2027 Graduate Software Engineer','grad','Australian/NZ Citizenship or PR','https://www.vivcourt.com/job-listings/?gh_jid=4653802006&gh_src=1v9yzsjq6uss','https://www.vivcourt.com/job-listings/?gh_jid=4653802006',ARRAY['Sydney, Australia'],'Message Nathan Dang-Huynh for a potential referral' FROM app.job_companies WHERE lower(trim(name))=lower('Vivcourt')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'2026/2027 Quant Trader Internship','intern','Australian/NZ Citizenship or PR','https://www.vivcourt.com/job-listings/?gh_jid=4653820006','https://www.vivcourt.com/job-listings/?gh_jid=4653820006',ARRAY['Sydney, Australia'],'Message Nathan Dang-Huynh for a potential referral' FROM app.job_companies WHERE lower(trim(name))=lower('Vivcourt')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineer – Intern (Australia)','intern','','https://www.citadelsecurities.com/careers/details/software-engineer-intern-australia/','https://www.citadelsecurities.com/careers/details/software-engineer-intern-australia/',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Citadel Securities')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Campus Software Engineer (Dec 2026 - Feb 2027 Intern)','intern','','https://www.jumptrading.com/hr/job?gh_jid=7543377','https://www.jumptrading.com/hr/job?gh_jid=7543377',ARRAY['Sydney','Singapore'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Jump Trading')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineer (C++ or Python) – 2026 Grads','grad','','https://www.hudsonrivertrading.com/hrt-job/software-engineer-c-or-python-2026-grads-3/','https://www.hudsonrivertrading.com/hrt-job/software-engineer-c-or-python-2026-grads-3/',ARRAY['Singapore'],'' FROM app.job_companies WHERE lower(trim(name))=lower('HRT')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineering Internship – Summer 2026','intern','','https://www.hudsonrivertrading.com/hrt-job/software-engineering-internship-summer-2026-2/','https://www.hudsonrivertrading.com/hrt-job/software-engineering-internship-summer-2026-2/',ARRAY['Singapore'],'' FROM app.job_companies WHERE lower(trim(name))=lower('HRT')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineer','grad','','https://www.janestreet.com/join-jane-street/position/8140274002/','https://www.janestreet.com/join-jane-street/position/8140274002/',ARRAY['Hong Kong, China'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Jane Street')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineer Internship, December-February','intern','','https://www.janestreet.com/join-jane-street/position/7582902002/','https://www.janestreet.com/join-jane-street/position/7582902002/',ARRAY['Hong Kong, China'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Jane Street')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineer Intern 2026/27','intern','Distinction average','https://www.imc.com/ap/careers/jobs/4629144101','https://www.imc.com/ap/careers/jobs/4629144101',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('IMC')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Career Kickstarter (Leads into 2027 Grad)','grad','','https://optiver.com/recruitment-events/career-kickstarter-sydney/','https://optiver.com/recruitment-events/career-kickstarter-sydney/',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Optiver')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Developer Internship – 2026-27','intern','','https://optiver.com/working-at-optiver/career-opportunities/7985973002/','https://optiver.com/working-at-optiver/career-opportunities/7985973002/',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Optiver')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Development Internship: November 2026','intern','','https://careers.sig.com/job/9615/Software-Development-Internship-November-2026','https://careers.sig.com/job/9615/Software-Development-Internship-November-2026',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Susquehanna')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Graduate Software Developer: 2027','grad','','https://careers.sig.com/job/9476/Graduate-Software-Developer-2027','https://careers.sig.com/job/9476/Graduate-Software-Developer-2027',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Susquehanna')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'2026 Australia Software Engineering Internship','intern','','https://www.linkedin.com/jobs/view/4365804755/','https://www.linkedin.com/jobs/view/4365804755/',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('The Trade Desk')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Global Technology Summer Analyst (Core Technology Infrastructure) - 2026/27 - Sydney','intern','','https://careers.bankofamerica.com/en-us/students/job-detail/14089/global-technology-summer-analyst-core-technology-infrastructure-2026-27-sydney-sydney-australia','https://careers.bankofamerica.com/en-us/students/job-detail/14089/global-technology-summer-analyst-core-technology-infrastructure-2026-27-sydney-sydney-australia',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Bank of America')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Software Engineer 2','grad','','https://www.linkedin.com/jobs/view/4379255325/','https://www.linkedin.com/jobs/view/4379255325/',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('MongoDB')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'2026/27 Software Engineering Intern Program','intern','','https://jobs.ashbyhq.com/airwallex/98cef827-ceb6-4caa-a2fd-e4d6e8c7d2a9/application','https://jobs.ashbyhq.com/airwallex/98cef827-ceb6-4caa-a2fd-e4d6e8c7d2a9/application',ARRAY['Sydney, Australia','Melbourne, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Airwallex')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

INSERT INTO app.job_positions(company_id,title,job_type,eligibility,application_url,normalized_url,locations,notes)
SELECT id,'Intern Engineer','intern','','https://jobs.dolby.com/careers?domain=dolby.com&triggerGoButton=false&start=0&pid=40606890&sort_by=timestamp&filter_seniority=0','https://jobs.dolby.com/careers?domain=dolby.com&filter_seniority=0&pid=40606890&sort_by=timestamp&start=0&triggerGoButton=false',ARRAY['Sydney, Australia'],'' FROM app.job_companies WHERE lower(trim(name))=lower('Dolby')
ON CONFLICT (normalized_url) WHERE merged_into_id IS NULL DO NOTHING;

-- +goose Down
-- Do not delete listings that staff or members may have used since import.
SELECT 1;
